package boltz

import (
	"fmt"

	"github.com/bolt-observer/agent/entities"
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

type FsmFn[I FsmInGetter, O FsmOutGetter] func(data I) O

type Fsm[I FsmInGetter, O FsmOutGetter, S State] struct {
	States map[S]func(data I) O
}

var Machine = &Fsm[FsmIn, FsmOut, State]{
	States: map[State]func(data FsmIn) FsmOut{
		None: FsmNone,

		SwapFailed:  FsmSwapFailed,
		SwapSuccess: FsmSwapSuccess,

		InitialNormal:        FsmInitialNormal,
		OnChainFundsSent:     FsmOnChainFundsSent,
		RedeemLockedFunds:    FsmRedeemLockedFunds,
		RedeemingLockedFunds: FsmRedeemingLockedFunds,
		VerifyFundsReceived:  FsmVerifyFundsReceived,
	},
}

type FsmIn struct {
	BoltzPlugin *Plugin
	SwapData    SwapData
	MsgCallback entities.MessageCallback
}

type FsmInGetter interface {
	Get() FsmIn
}

func (f FsmIn) Get() FsmIn {
	return f
}

func (f FsmIn) GetJobID() int32 {
	return f.SwapData.JobID
}

type FsmOut struct {
	Error     error
	NextState State
}

func (f FsmOut) Get() FsmOut {
	return f
}

type FsmOutGetter interface {
	Get() FsmOut
}

func (fsm Fsm[I, O, S]) FsmWrap(f func(data I) O) func(data I) O {
	return func(in I) O {

		realIn := in.Get()
		out := f(in)
		realOut := out.Get()

		if realOut.Error != nil {
			realOut.NextState = SwapFailed
			realIn.MsgCallback(entities.PluginMessage{
				JobID:      realIn.GetJobID(),
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
			err := changeState(realIn, realOut.NextState)
			if err != nil {
				realOut.NextState = SwapFailed
			}

		}

		return out
	}
}

func changeState(in FsmIn, state State) error {
	in.SwapData.State = state
	err := in.BoltzPlugin.db.Insert(in.SwapData, in.SwapData.JobID)
	if err != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      in.GetJobID(),
			Message:    fmt.Sprintf("Could not change state to %v %v", state, err),
			IsError:    true,
			IsFinished: true,
		})
	}
	return err
}

func (fsm Fsm[I, O, S]) FsmEval(
	in I,
	initial, fail FsmFn[I, O],
	wrap func(f FsmFn[I, O]) FsmFn[I, O]) O {

	var (
		fun func(data I) O
		ok  bool
		out O
	)

	if wrap != nil {
		fun = wrap(initial)
	} else {
		fun = initial
	}
	for {
		if wrap != nil {
			out = wrap(fun)(in)
		} else {
			out = fun(in)
		}

		realOut := out.Get()
		if realOut.NextState == None {
			return out
		}
		s := S(realOut.NextState)
		fun, ok = fsm.States[s]
		if !ok {
			if fail == nil {
				return out
			}
			fun = fail
		}
	}
}

func FsmNone(in FsmIn) FsmOut {
	panic("Invalid state reached")
}

func FsmSwapFailed(in FsmIn) FsmOut {
	in.MsgCallback(entities.PluginMessage{
		JobID:      in.GetJobID(),
		Message:    fmt.Sprintf("Swap %d failed\n", in.GetJobID()),
		IsError:    true,
		IsFinished: true,
	})
	return FsmOut{}
}

func FsmSwapSuccess(in FsmIn) FsmOut {
	in.MsgCallback(entities.PluginMessage{
		JobID:      in.GetJobID(),
		Message:    fmt.Sprintf("Swap %d succeeded\n", in.GetJobID()),
		IsError:    false,
		IsFinished: true,
	})
	return FsmOut{}
}
