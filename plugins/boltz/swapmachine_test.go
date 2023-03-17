//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning"
	lnapi "github.com/bolt-observer/agent/lightning"
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
	CurrentState State
	In           FsmIn
}

func NewFakeSwapMachine(plugin *Plugin, nodeDataInvalidator entities.Invalidatable, jobDataToSwapData JobDataToSwapDataFn, lnAPI agent_entities.NewAPICall) *FakeSwapMachine {
	s := &FakeSwapMachine{}
	s.Machine = &Fsm[FsmIn, FsmOut, State]{States: make(map[State]func(data FsmIn) FsmOut)}
	s.BoltzPlugin = plugin
	s.NodeDataInvalidator = nodeDataInvalidator
	s.JobDataToSwapData = jobDataToSwapData
	s.LnAPI = lnAPI

	s.Machine.States[SwapFailed] = func(in FsmIn) FsmOut { s.CurrentState = SwapFailed; s.In = in; return FsmOut{} }
	s.Machine.States[SwapSuccess] = func(in FsmIn) FsmOut { s.CurrentState = SwapSuccess; s.In = in; return FsmOut{} }

	s.Machine.States[InitialForward] = func(in FsmIn) FsmOut { s.CurrentState = InitialForward; s.In = in; return FsmOut{} }
	s.Machine.States[InitialReverse] = func(in FsmIn) FsmOut { s.CurrentState = InitialReverse; s.In = in; return FsmOut{} }

	return s
}

func mkFakeLndAPI() agent_entities.NewAPICall {
	return func() (lnapi.LightingAPICalls, error) {
		return &lnapi.MockLightningAPI{}, nil
	}
}

func TestNextRoundNotNeeded(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    &boltz.Boltz{URL: "https://testapi.boltz.exchange"},
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
		Limits: &SwapLimits{
			MaxAttempts: 10,
		},
	}

	invalidatable := &FakeInvalidatable{}

	noFun := func(ctx context.Context, limits *SwapLimits, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error) {
		return nil, ErrNoNeedToDoAnything
	}

	in := FsmIn{
		SwapData: &SwapData{
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
		BoltzAPI:    &boltz.Boltz{URL: "https://testapi.boltz.exchange"},
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
		Limits: &SwapLimits{
			MaxAttempts: 10,
		},
	}

	sd := &SwapData{
		Attempt: 1,
		State:   SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits *SwapLimits, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error) {
		sd.State = InitialForward
		return sd, nil
	}

	in := FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(p, invalidatable, fun, p.LnAPI)
	o := s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, false, invalidatable.WasCalled)
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 2, s.In.SwapData.Attempt)
	assert.Equal(t, InitialForward, s.In.SwapData.State)

	// Now fast forward to last attempt

	sd.State = SwapSuccess
	sd.Attempt = p.Limits.MaxAttempts
	in.SwapData = sd

	o = s.nextRound(in)
	assert.NoError(t, o.Error)

	assert.Equal(t, true, invalidatable.WasCalled)
}

func TestNextRoundJobConversionFails(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    &boltz.Boltz{URL: "https://testapi.boltz.exchange"},
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
	}

	sd := &SwapData{
		Attempt: 1,
		State:   SwapSuccess,
	}

	invalidatable := &FakeInvalidatable{}

	fun := func(ctx context.Context, limits *SwapLimits, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error) {
		return nil, ErrInvalidArguments
	}

	in := FsmIn{
		SwapData: sd,
	}

	s := NewFakeSwapMachine(p, invalidatable, fun, p.LnAPI)
	o := s.nextRound(in)

	assert.ErrorIs(t, o.Error, ErrInvalidArguments)
}
