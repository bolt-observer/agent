//go:build plugins
// +build plugins

package boltz

type FsmFn[I FsmInGetter, O FsmOutGetter] func(data I) O

type Fsm[I FsmInGetter, O FsmOutGetter, S State] struct {
	States map[S]func(data I) O
}

type FsmInGetter interface {
	Get() FsmIn
}

func (f FsmIn) Get() FsmIn {
	return f
}

func (f FsmIn) GetJobID() JobID {
	return f.SwapData.JobID
}

func (f FsmIn) GetUniqueJobID() string {
	return f.SwapData.GetUniqueJobID()
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

func (fsm Fsm[I, O, S]) FsmEval(
	in I,
	initial, fail FsmFn[I, O]) O {

	var (
		fun func(data I) O
		ok  bool
		out O
	)

	fun = initial
	for {
		out = fun(in)

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
