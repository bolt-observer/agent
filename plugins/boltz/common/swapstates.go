//go:build plugins
// +build plugins

package common

import "github.com/bolt-observer/agent/entities"

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

func (s *State) IsFinal() bool {
	return *s == SwapFailed || *s == SwapSuccess
}

// FsmIn is the input to each state
type FsmIn struct {
	SwapData    *SwapData
	MsgCallback entities.MessageCallback
}

// To satisfy interface
func (i FsmIn) GetSwapData() *SwapData {
	return i.SwapData
}
