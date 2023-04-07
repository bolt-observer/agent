//go:build plugins
// +build plugins

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	A State = 1001
	B State = 1002
	C State = 1003
)

func FsmA(in TestFsmIn) TestFsmOut {
	*in.Trail += "A"

	r := TestFsmOut{}
	r.NextState = B

	return r
}

func FsmB(in TestFsmIn) TestFsmOut {
	*in.Trail += "B"

	r := TestFsmOut{}
	r.NextState = C

	return r
}

func FsmC(in TestFsmIn) TestFsmOut {
	*in.Trail += "C"

	r := TestFsmOut{}
	return r
}

type TestFsmOut struct {
	Trail *string

	FsmOut
}

func (f TestFsmIn) Get() FsmIn {
	return f.FsmIn
}

func (f TestFsmOut) Get() FsmOut {
	return f.FsmOut
}

type TestFsmIn struct {
	Trail *string

	FsmIn
}

func TestFsmEval(t *testing.T) {
	trail := ""
	fsm := &Fsm[TestFsmIn, TestFsmOut, State]{
		States: map[State]func(data TestFsmIn) TestFsmOut{
			A: FsmA,
			B: FsmB,
			C: FsmC,
		},
	}

	fsm.FsmEval(TestFsmIn{Trail: &trail}, FsmA, nil)
	assert.Equal(t, "ABC", trail)
}
