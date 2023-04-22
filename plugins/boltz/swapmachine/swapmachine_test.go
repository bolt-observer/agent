//go:build plugins
// +build plugins

package swapmachine

import (
	"context"
	"testing"
	"time"

	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning"
	api "github.com/bolt-observer/agent/lightning"

	lnapi "github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	data "github.com/bolt-observer/agent/plugins/boltz/data"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
)

type FakeInvalidatable struct {
	WasCalled bool
}

func (f *FakeInvalidatable) Invalidate() error {
	f.WasCalled = true
	return nil
}

type FakeSwapMachine struct {
	SwapMachine
	CurrentState common.State
	In           common.FsmIn
}

func NewFakeSwapMachine(pd data.PluginData, nodeDataInvalidator entities.Invalidatable, jobDataToSwapData common.JobDataToSwapDataFn, lnAPI api.NewAPICall) *FakeSwapMachine {
	s := &FakeSwapMachine{}
	s.Machine = &common.Fsm[common.FsmIn, common.FsmOut, common.State]{States: make(map[common.State]func(data common.FsmIn) common.FsmOut)}

	s.ReferralCode = pd.ReferralCode
	s.ChainParams = pd.ChainParams
	s.Filter = pd.Filter
	s.CryptoAPI = pd.CryptoAPI
	s.Redeemer = pd.Redeemer
	s.ReverseRedeemer = pd.ReverseRedeemer
	s.BoltzAPI = pd.BoltzAPI

	s.NodeDataInvalidator = nodeDataInvalidator
	s.JobDataToSwapData = jobDataToSwapData
	s.LnAPI = lnAPI
	s.AllowChanCreation = true
	s.PrivateChanCreation = false

	s.Machine.States[common.SwapFailed] = func(in common.FsmIn) common.FsmOut {
		s.CurrentState = common.SwapFailed
		s.In = in
		return common.FsmOut{}
	}
	s.Machine.States[common.SwapSuccess] = func(in common.FsmIn) common.FsmOut {
		s.CurrentState = common.SwapSuccess
		s.In = in
		return common.FsmOut{}
	}

	s.Machine.States[common.InitialForward] = func(in common.FsmIn) common.FsmOut {
		s.CurrentState = common.InitialForward
		s.In = in
		return common.FsmOut{}
	}
	s.Machine.States[common.InitialReverse] = func(in common.FsmIn) common.FsmOut {
		s.CurrentState = common.InitialReverse
		s.In = in
		return common.FsmOut{}
	}
	s.Machine.States[common.SwapInvoiceCouldNotBePaid] = s.FsmSwapInvoiceCouldNotBePaid

	return s
}

func mkFakeLndAPI() api.NewAPICall {
	return func() (lnapi.LightingAPICalls, error) {
		return &lnapi.MockLightningAPI{}, nil
	}
}

func TestNextRoundNotNeeded(t *testing.T) {
	pd := data.PluginData{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
		ChangeStateFn: data.ChangeStateFn(func(in common.FsmIn, state common.State) error {
			in.SwapData.State = state
			return nil
		}),
		GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
			return 100 * time.Millisecond
		}),
	}

	invalidatable := &FakeInvalidatable{}

	noFun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		return nil, common.ErrNoNeedToDoAnything
	}

	in := common.FsmIn{
		SwapData: &common.SwapData{
			Attempt: 1,
		},
	}

	s := NewFakeSwapMachine(pd, invalidatable, noFun, mkFakeLndAPI())
	o := s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, true, invalidatable.WasCalled)
}

func TestNextRoundNeeded(t *testing.T) {
	pd := data.PluginData{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
		ChangeStateFn: data.ChangeStateFn(func(in common.FsmIn, state common.State) error {
			in.SwapData.State = state
			return nil
		}),
		GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
			return 100 * time.Millisecond
		}),
	}

	sd := &common.SwapData{
		SwapLimits: pd.Limits,
		Attempt:    1,
		State:      common.SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		sd.State = common.InitialForward
		sd.SwapLimits = limits
		return sd, nil
	}

	in := common.FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(pd, invalidatable, fun, mkFakeLndAPI())
	o := s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, false, invalidatable.WasCalled)
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 2, s.In.SwapData.Attempt)
	assert.Equal(t, common.InitialForward, s.In.SwapData.State)

	// Now fast forward to last attempt

	sd.State = common.SwapSuccess
	sd.Attempt = pd.Limits.MaxAttempts
	in.SwapData = sd

	o = s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, true, invalidatable.WasCalled)
}

func TestNextRoundJobConversionFails(t *testing.T) {
	pd := data.PluginData{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
		ChangeStateFn: data.ChangeStateFn(func(in common.FsmIn, state common.State) error {
			in.SwapData.State = state
			return nil
		}),
		GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
			return 100 * time.Millisecond
		}),
	}

	sd := &common.SwapData{
		Attempt: 1,
		State:   common.SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		return nil, common.ErrInvalidArguments
	}

	in := common.FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(pd, invalidatable, fun, mkFakeLndAPI())
	o := s.nextRound(in)

	assert.ErrorIs(t, o.Error, common.ErrInvalidArguments)
}

func TestSwapCouldNotBePaid(t *testing.T) {
	pd := data.PluginData{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
		ChangeStateFn: data.ChangeStateFn(func(in common.FsmIn, state common.State) error {
			in.SwapData.State = state
			return nil
		}),
		GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
			return 100 * time.Millisecond
		}),
	}

	sd := &common.SwapData{
		Attempt:      1,
		State:        common.SwapInvoiceCouldNotBePaid,
		ExpectedSats: 100000,
		SwapLimits: common.SwapLimits{
			BackOffAmount: 0.8,
			MaxSwap:       200000,
			MinSwap:       10000,
			DefaultSwap:   10000,
			MaxAttempts:   10,
		},
		OriginalJobData: common.DummyJobData,
	}

	invalidatable := &FakeInvalidatable{}

	called := false
	fun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		assert.Equal(t, uint64(80000), limits.MaxSwap)
		called = true
		return &common.SwapData{
			Attempt:    2,
			SwapLimits: limits,
		}, nil
	}

	in := common.FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(pd, invalidatable, fun, mkFakeLndAPI())
	s.Eval(in, sd.State)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, true, called)
}
