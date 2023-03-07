package boltz

import (
	"encoding/json"
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrCouldNotParseJobData  = Error("could not parse job data")
	ErrUnsupportedTargetType = Error("unsupported target type")
)

type TargetType string

const (
	OutboundLiqudityNodeTarget   TargetType = "OutboundLiqudityNodeTarget"
	InboundLiqudityNodeTarget    TargetType = "InboundLiqudityNodeTarget"
	InboundLiqudityChannelTarget TargetType = "InboundLiqudityChannelTarget"
)

type JobData struct {
	ID         int32      `json:"id,omitempty"`
	Target     TargetType `json:"target"`
	Percentage float64    `json:"percentage,omitempty"`
	ChannelId  uint64     `json:"channel_id,omitempty"`
}

type SwapData struct {
	JobID             JobID
	Attempt           int // Not used yet
	BoltzID           string
	State             State
	TimoutBlockHeight uint32
	Script            string
	Sats              uint64
	Address           string

	// Normal swap
	Invoice             string
	LockupTransactionId string
	OnChainFundsSent    bool

	// Reverse swap
	ReverseInvoice      string
	ReverseMinerInvoice string
	ChanIdsToUse        []uint64
}

func ParseJobData(id int32, bytes []byte) (*JobData, error) {
	var jd JobData
	err := json.Unmarshal(bytes, &jd)
	if err != nil {
		return nil, ErrCouldNotParseJobData
	}

	jd.ID = id
	switch jd.Target {
	case OutboundLiqudityNodeTarget:
		fallthrough
	case InboundLiqudityNodeTarget:
		if jd.Percentage <= 0 || jd.Percentage >= 100 {
			return nil, ErrCouldNotParseJobData
		}
		return &jd, nil
	case InboundLiqudityChannelTarget:
		if jd.Percentage <= 0 || jd.Percentage >= 100 {
			return nil, ErrCouldNotParseJobData
		}
		if jd.ChannelId == 0 {
			return nil, ErrCouldNotParseJobData
		}
		return &jd, nil
	}
	// Possibly deserialize to something extending JobData for more complicate scenarios

	return nil, ErrUnsupportedTargetType
}
