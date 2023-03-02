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
	State             State
	TimoutBlockHeight uint32

	// Normal swap
	Invoice             string
	LockupTransactionId string
	OnChainFundsSent    bool

	// Reverse swap
	ReverseInvoice string
	ReverseMinerInvoice string

}

type State int

const (
	Initial State = iota
	OnChainFundsSent


	Successful
	ServerError
	ClientError
	// Client refunded locked coins after the HTLC timed out
	Refunded
	// Client noticed that the HTLC timed out but didn't find any outputs to refund
	Abandoned
)

type Type int

const (
	Normal Type = iota
	Reverse
)

type Error string

func (e Error) Error() string { return string(e) }

const ErrCouldNotParseJobData = Error("could not parse job data")
const ErrInvalidArguments = Error("invalid arguments")
