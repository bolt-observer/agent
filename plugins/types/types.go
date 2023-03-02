package types

type JobData struct {
	Target    string
	ChannelID string
	Amount    int64
}

type SwapData struct {
	JobID             int32
	Attempt           int
	BoltzID           string
	Type              Type
	State             State
	TimoutBlockHeight uint32

	// Normal swap
	Invoice             string
	LockupTransactionId string
	OnChainFundsSent    bool

	// Reverse swap
	ReverseInvoice      string
	ReverseMinerInvoice string
}

type State int

const (
	Initial State = iota

	SwapFailed
	SwapSuccess

	OnChainFundsSent
	RedeemLockedFunds
	RedeemingLockedFunds
	VerifyFundsReceived

	ReverseSwapCreated
	ClaimReverseFunds
)

func (s State) isFinal() bool {
	return s == SwapFailed || s == SwapSuccess
}

type Type int

const (
	Normal Type = iota
	Reverse
)

type Error string

func (e Error) Error() string { return string(e) }

const ErrCouldNotParseJobData = Error("could not parse job data")
const ErrInvalidArguments = Error("invalid arguments")
