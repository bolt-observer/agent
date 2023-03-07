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

type State int

const (
	None State = iota

	InitialNormal
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

func (s *State) isFinal() bool {
	return *s == SwapFailed || *s == SwapSuccess
}

type FsmIn struct {
	SwapData    SwapData
	MsgCallback entities.MessageCallback
}

func (i FsmIn) GetSwapData() SwapData {
	return i.SwapData
}

func (b *Plugin) changeState(in FsmIn, state State) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	in.SwapData.State = state
	b.jobs[int32(in.SwapData.JobID)] = in.SwapData
	err := b.db.Insert(in.SwapData, in.SwapData.JobID)
	if err != nil {
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
	in.MsgCallback(entities.PluginMessage{
		JobID:      int32(in.GetJobID()),
		Message:    msg,
		IsError:    false,
		IsFinished: false,
	})
}

func (s *SwapMachine) GetSleepTime(in FsmIn) time.Duration {
	if s.BoltzPlugin.ChainParams.Name == "mainnet" {
		return 1 * time.Minute
	}
	return 5 * time.Second
}

func (s *SwapMachine) FsmNone(in FsmIn) FsmOut {
	panic("Invalid state reached")
}

func (s *SwapMachine) FsmSwapFailed(in FsmIn) FsmOut {
	in.MsgCallback(entities.PluginMessage{
		JobID:      int32(in.GetJobID()),
		Message:    fmt.Sprintf("Swap %d failed\n", in.GetJobID()),
		IsError:    true,
		IsFinished: true,
	})
	return FsmOut{}
}

func (s *SwapMachine) FsmSwapSuccess(in FsmIn) FsmOut {
	in.MsgCallback(entities.PluginMessage{
		JobID:      int32(in.GetJobID()),
		Message:    fmt.Sprintf("Swap %d succeeded\n", in.GetJobID()),
		IsError:    false,
		IsFinished: true,
	})
	return FsmOut{}
}

func (s *SwapMachine) RedeemerCallback(data FsmIn, success bool) {
	sd := data.GetSwapData()
	if sd.State == RedeemingLockedFunds {
		if !success {
			s.Eval(data, SwapFailed)
		}
	} else if sd.State == ClaimReverseFunds {
		if success {
			s.Eval(data, SwapSuccess)
		}
	}
}

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
			if err != nil {
				realOut.NextState = SwapFailed
			}

		}

		return out
	}
}

type SwapMachine struct {
	Machine     *Fsm[FsmIn, FsmOut, State]
	BoltzPlugin *Plugin
}

func NewSwapMachine(plugin *Plugin) *SwapMachine {
	s := &SwapMachine{
		Machine:     &Fsm[FsmIn, FsmOut, State]{States: make(map[State]func(data FsmIn) FsmOut)},
		BoltzPlugin: plugin,
	}

	s.Machine.States[SwapFailed] = FsmWrap(s.FsmSwapFailed, plugin)
	s.Machine.States[SwapSuccess] = FsmWrap(s.FsmSwapSuccess, plugin)

	s.Machine.States[InitialNormal] = FsmWrap(s.FsmInitialNormal, plugin)
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
	initF := s.Machine.States[initial]
	fail := s.FsmNone
	return s.Machine.FsmEval(in, initF, fail)
}
