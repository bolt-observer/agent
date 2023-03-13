//go:build plugins
// +build plugins

package boltz

import (
	"fmt"
	"time"

	"github.com/bolt-observer/agent/entities"
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
	return FsmOut{}
}

func (s *SwapMachine) FsmSwapSuccess(in FsmIn) FsmOut {
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    fmt.Sprintf("Swap %d succeeded (dry run: %v)", in.GetJobID(), in.SwapData.IsDryRun),
			IsError:    false,
			IsFinished: true,
		})
	}
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
			go s.Eval(data, SwapSuccess)
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
)

func (s State) String() string {
	return []string{"None", "InitialForward", "InitialReverse", "SwapFaied", "SwapSuccess", "OnChainFundsSent",
		"RedeemLockedFunds", "RedeemingLockedFunds", "VerifyFundsReceived", "ReverseSwapCreated", "ClaimReverseFunds"}[s]
}

func (s *State) isFinal() bool {
	return *s == SwapFailed || *s == SwapSuccess
}

// Swapmachine is a finite state machine used for swaps.
type SwapMachine struct {
	Machine     *Fsm[FsmIn, FsmOut, State]
	BoltzPlugin *Plugin
}

func NewSwapMachine(plugin *Plugin) *SwapMachine {
	s := &SwapMachine{Machine: &Fsm[FsmIn, FsmOut, State]{States: make(map[State]func(data FsmIn) FsmOut)},
		// TODO: instead of BoltzPlugin this should be a bit more granular
		BoltzPlugin: plugin,
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
