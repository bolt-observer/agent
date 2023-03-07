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
	OutboundLiqudityNodePercent   TargetType = "OutboundLiqudityNodePercent"
	InboundLiqudityNodePercent    TargetType = "InboundLiqudityNodePercent"
	InboundLiqudityChannelPercent TargetType = "InboundLiqudityChannelPercent"
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

	// Reverse swap
	ReverseInvoice   string
	ReverseChannelId uint64 // 0 means node level
	//ReverseMinerInvoice string - not supported
	ChanIdsToUse []uint64
	ExpectedSats uint64
}

func ParseJobData(id int32, bytes []byte) (*JobData, error) {
	var jd JobData
	err := json.Unmarshal(bytes, &jd)
	if err != nil {
		return nil, ErrCouldNotParseJobData
	}

	jd.ID = id
	switch jd.Target {
	case OutboundLiqudityNodePercent:
		fallthrough
	case InboundLiqudityNodePercent:
		if jd.Percentage <= 0 || jd.Percentage >= 100 {
			return nil, ErrCouldNotParseJobData
		}
		return &jd, nil
	case InboundLiqudityChannelPercent:
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
