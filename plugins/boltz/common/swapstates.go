//go:build plugins
// +build plugins

package common

import "github.com/bolt-observer/agent/entities"

// SwapType enum.
type SwapType string

const (
	Unknown SwapType = "unknown"
	Forward SwapType = "forward"
	Reverse SwapType = "reverse"
)

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

func (s State) IsFinal() bool {
	return s == SwapFailed || s == SwapSuccess
}

func (s State) ToSwapType() SwapType {
	switch s {
	case InitialForward:
	case OnChainFundsSent:
	case RedeemLockedFunds:
	case RedeemingLockedFunds:
	case VerifyFundsReceived:
		return Forward
	case InitialReverse:
	case ReverseSwapCreated:
	case ClaimReverseFunds:
	case SwapClaimed:
		return Reverse
	}

	return Unknown
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
