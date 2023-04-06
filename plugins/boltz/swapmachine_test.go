//go:build plugins
// +build plugins

package boltz

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

func NewFakeSwapMachine(plugin *Plugin, nodeDataInvalidator entities.Invalidatable, jobDataToSwapData common.JobDataToSwapDataFn, lnAPI api.NewAPICall) *FakeSwapMachine {
	s := &FakeSwapMachine{}
	s.Machine = &common.Fsm[common.FsmIn, common.FsmOut, common.State]{States: make(map[common.State]func(data common.FsmIn) common.FsmOut)}
	s.ReferralCode = plugin.ReferralCode
	s.ChainParams = plugin.ChainParams
	s.Filter = plugin.Filter
	s.CryptoAPI = plugin.CryptoAPI
	s.Redeemer = plugin.Redeemer
	s.ReverseRedeemer = plugin.ReverseRedeemer
	s.Limits = plugin.Limits
	s.BoltzAPI = plugin.BoltzAPI

	s.NodeDataInvalidator = nodeDataInvalidator
	s.JobDataToSwapData = jobDataToSwapData
	s.LnAPI = lnAPI

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

	return s
}

func mkFakeLndAPI() api.NewAPICall {
	return func() (lnapi.LightingAPICalls, error) {
		return &lnapi.MockLightningAPI{}, nil
	}
}

func TestNextRoundNotNeeded(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
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

	s := NewFakeSwapMachine(p, invalidatable, noFun, p.LnAPI)
	o := s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, true, invalidatable.WasCalled)
}

func TestNextRoundNeeded(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
	}

	sd := &common.SwapData{
		Attempt: 1,
		State:   common.SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		sd.State = common.InitialForward
		return sd, nil
	}

	in := common.FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(p, invalidatable, fun, p.LnAPI)
	o := s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, false, invalidatable.WasCalled)
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 2, s.In.SwapData.Attempt)
	assert.Equal(t, common.InitialForward, s.In.SwapData.State)

	// Now fast forward to last attempt

	sd.State = common.SwapSuccess
	sd.Attempt = p.Limits.MaxAttempts
	in.SwapData = sd

	o = s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, true, invalidatable.WasCalled)
}

func TestNextRoundJobConversionFails(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
	}

	sd := &common.SwapData{
		Attempt: 1,
		State:   common.SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits common.SwapLimits, jobData *common.JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*common.SwapData, error) {
		return nil, ErrInvalidArguments
	}

	in := common.FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(p, invalidatable, fun, p.LnAPI)
	o := s.nextRound(in)

	assert.ErrorIs(t, o.Error, ErrInvalidArguments)
}
