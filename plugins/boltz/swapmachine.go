//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"fmt"
	"time"

	"github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
)

const (
	BtcPair = "BTC/BTC"
	Btc     = "BTC"
)

// FsmIn is the input to each state
type FsmIn struct {
	SwapData    *SwapData
	MsgCallback entities.MessageCallback
}

// To satisfy interface
func (i FsmIn) GetSwapData() *SwapData {
	return i.SwapData
}

// changeState actually registers the state change
func (b *Plugin) changeState(in FsmIn, state State) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	in.SwapData.State = state
	b.jobs[int32(in.SwapData.JobID)] = in.SwapData
	err := b.db.Update(in.SwapData.JobID, in.SwapData)
	if err != nil && in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    fmt.Sprintf("Could not change state to %v %v", state, err),
			IsError:    true,
			IsFinished: true,
		})
	}
	return err
}

func log(in FsmIn, msg string) {
	glog.Infof("[Boltz] [%d] %s", in.GetJobID(), msg)
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    msg,
			IsError:    false,
			IsFinished: false,
		})
	}
}

func (s *SwapMachine) getSleepTime(in FsmIn) time.Duration {
	return s.BoltzPlugin.getSleepTime()
}

func (s *SwapMachine) FsmNone(in FsmIn) FsmOut {
	log(in, "Invalid state reached")
	panic("Invalid state reached")
}

func (s *SwapMachine) FsmSwapFailed(in FsmIn) FsmOut {
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    fmt.Sprintf("Swap %d failed", in.GetJobID()),
			IsError:    true,
			IsFinished: true,
		})
	}

	// Make sure after swap latest data is sent
	if s.NodeDataInvalidator != nil {
		s.NodeDataInvalidator.Invalidate()
	}
	return FsmOut{}
}

func (s *SwapMachine) FsmSwapSuccess(in FsmIn) FsmOut {
	// TODO: it would be great if we could calculate how much the swap actually cost us, but it is hard to do precisely
	// because redeemer might have claimed multiple funds
	if in.MsgCallback != nil {
		message := fmt.Sprintf("Swap %d (attempt %d) succeeded", in.GetJobID(), in.SwapData.Attempt)
		if in.SwapData.IsDryRun {
			message = fmt.Sprintf("Swap %d finished in dry-run mode (no funds were used)", in.GetJobID())
		}
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    message,
			IsError:    false,
			IsFinished: in.SwapData.IsDryRun,
		})
	}

	if in.SwapData.IsDryRun {
		return FsmOut{}
	}

	return s.nextRound(in)
}

func (s *SwapMachine) nextRound(in FsmIn) FsmOut {
	ctx := context.Background()

	lnConnection, err := s.LnAPI()
	if err != nil {
		return FsmOut{Error: err}
	}
	if lnConnection == nil {
		return FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	sd, err := s.JobDataToSwapData(ctx, s.BoltzPlugin.Limits, &in.SwapData.OriginalJobData, in.MsgCallback, lnConnection, s.BoltzPlugin.Filter)

	if err == ErrNoNeedToDoAnything {
		if in.MsgCallback != nil {
			message := fmt.Sprintf("Swap %d overall finished in attempt %d", in.GetJobID(), in.SwapData.Attempt)
			in.MsgCallback(entities.PluginMessage{
				JobID:      int32(in.GetJobID()),
				Message:    message,
				IsError:    false,
				IsFinished: true,
			})
		}

		// Make sure after swap latest data is sent
		if s.NodeDataInvalidator != nil {
			s.NodeDataInvalidator.Invalidate()
		}

		return FsmOut{}
	} else if err != nil {
		return FsmOut{Error: err}
	}

	// TODO: does this make sense? Maybe we should start multiple swaps in parallel at the begining
	sd.Attempt = in.SwapData.Attempt + 1
	if sd.Attempt > s.BoltzPlugin.Limits.MaxAttempts {
		if in.MsgCallback != nil {
			message := fmt.Sprintf("Swap %d aborted after attempt %d/%d", in.GetJobID(), in.SwapData.Attempt, s.BoltzPlugin.Limits.MaxAttempts)
			in.MsgCallback(entities.PluginMessage{
				JobID:      int32(in.GetJobID()),
				Message:    message,
				IsError:    true,
				IsFinished: true,
			})
		}

		// Make sure after swap latest data is sent
		if s.NodeDataInvalidator != nil {
			s.NodeDataInvalidator.Invalidate()
		}

		return FsmOut{}
	}

	in.SwapData = sd
	go s.Eval(in, sd.State)

	return FsmOut{}
}

func (s *SwapMachine) RedeemedCallback(data FsmIn, success bool) {
	sd := data.GetSwapData()
	if sd.State == RedeemingLockedFunds {
		// If we are redeeming locked funds this means by definition our swap failed
		// so when redeemer was able to recover the funds we can transition to final state
		// else we stay in RedeemingLockedFunds and continue with it until eternity
		if success {
			go s.Eval(data, SwapFailed)
		} else {
			go func() {
				time.Sleep(s.getSleepTime(data))
				s.Eval(data, RedeemingLockedFunds)
			}()
		}
	} else if sd.State == ClaimReverseFunds {
		if success {
			go s.Eval(data, SwapClaimed)
		} else {
			go s.Eval(data, SwapFailed)
		}
	} else {
		log(data, fmt.Sprintf("Received redeemed callback in wrong state %v", sd.State))
	}
}

// FsmWrap will just wrap a normal state machine function and give it the ability to transition states based on return values
func FsmWrap[I FsmInGetter, O FsmOutGetter](f func(data I) O, b *Plugin) func(data I) O {
	return func(in I) O {

		realIn := in.Get()
		out := f(in)
		realOut := out.Get()

		if realOut.Error != nil {
			realOut.NextState = SwapFailed
			realIn.MsgCallback(entities.PluginMessage{
				JobID:      int32(realIn.GetJobID()),
				Message:    fmt.Sprintf("Error %v happened", realOut.Error),
				IsError:    true,
				IsFinished: true,
			})
			return out
		}

		if realIn.SwapData.State.isFinal() {
			return out
		}
		if realOut.NextState != None {
			err := b.changeState(realIn, realOut.NextState)
			realIn.MsgCallback(entities.PluginMessage{
				JobID:      int32(realIn.GetJobID()),
				Message:    fmt.Sprintf("Transitioning to state %v", realOut.NextState),
				IsError:    false,
				IsFinished: false,
			})
			if err != nil {
				realOut.NextState = SwapFailed
			}
		}

		return out
	}
}

// State
type State int

const (
	None State = iota

	InitialForward
	InitialReverse

	SwapFailed
	SwapSuccess

	OnChainFundsSent
	RedeemLockedFunds
	RedeemingLockedFunds
	VerifyFundsReceived

	ReverseSwapCreated
	ClaimReverseFunds
	SwapClaimed
)

func (s State) String() string {
	return []string{"None", "InitialForward", "InitialReverse", "SwapFaied", "SwapSuccess", "OnChainFundsSent",
		"RedeemLockedFunds", "RedeemingLockedFunds", "VerifyFundsReceived", "ReverseSwapCreated", "ClaimReverseFunds", "SwapClaimed"}[s]
}

func (s *State) isFinal() bool {
	return *s == SwapFailed || *s == SwapSuccess
}

// Swapmachine is a finite state machine used for swaps.
type SwapMachine struct {
	Machine *Fsm[FsmIn, FsmOut, State]
	// TODO: we should not be referencing plugin here
	BoltzPlugin         *Plugin
	NodeDataInvalidator entities.Invalidatable
	JobDataToSwapData   JobDataToSwapDataFn
	LnAPI               api.NewAPICall
}

func NewSwapMachine(plugin *Plugin, nodeDataInvalidator entities.Invalidatable, jobDataToSwapData JobDataToSwapDataFn, lnAPI api.NewAPICall) *SwapMachine {
	s := &SwapMachine{Machine: &Fsm[FsmIn, FsmOut, State]{States: make(map[State]func(data FsmIn) FsmOut)},
		BoltzPlugin:         plugin,
		NodeDataInvalidator: nodeDataInvalidator,
		JobDataToSwapData:   jobDataToSwapData,
		LnAPI:               lnAPI,
	}

	s.Machine.States[SwapFailed] = FsmWrap(s.FsmSwapFailed, plugin)
	s.Machine.States[SwapSuccess] = FsmWrap(s.FsmSwapSuccess, plugin)

	s.Machine.States[InitialForward] = FsmWrap(s.FsmInitialForward, plugin)
	s.Machine.States[OnChainFundsSent] = FsmWrap(s.FsmOnChainFundsSent, plugin)
	s.Machine.States[RedeemLockedFunds] = FsmWrap(s.FsmRedeemLockedFunds, plugin)
	s.Machine.States[RedeemingLockedFunds] = FsmWrap(s.FsmRedeemingLockedFunds, plugin)
	s.Machine.States[VerifyFundsReceived] = FsmWrap(s.FsmVerifyFundsReceived, plugin)

	s.Machine.States[InitialReverse] = FsmWrap(s.FsmInitialReverse, plugin)
	s.Machine.States[ReverseSwapCreated] = FsmWrap(s.FsmReverseSwapCreated, plugin)
	s.Machine.States[ClaimReverseFunds] = FsmWrap(s.FsmClaimReverseFunds, plugin)
	s.Machine.States[SwapClaimed] = FsmWrap(s.FsmSwapClaimed, plugin)

	return s
}

func (s *SwapMachine) Eval(in FsmIn, initial State) FsmOut {
	initF, ok := s.Machine.States[initial]
	if !ok {
		return FsmOut{Error: fmt.Errorf("invalid initial state: %v", initial)}
	}
	fail := s.FsmNone
	return s.Machine.FsmEval(in, initF, fail)
}
