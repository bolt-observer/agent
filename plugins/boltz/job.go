//go:build plugins
// +build plugins

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
	OutboundLiquidityNodePercent   TargetType = "OutboundLiquidityNodePercent"
	InboundLiquidityNodePercent    TargetType = "InboundLiquidityNodePercent"
	InboundLiquidityChannelPercent TargetType = "InboundLiquidityChannelPercent"
)

// JobData struct comes from the request.
type JobData struct {
	ID         int32      `json:"id,omitempty"`
	Target     TargetType `json:"target"`
	Percentage float64    `json:"percentage,omitempty"`
	ChannelId  uint64     `json:"channel_id,omitempty"`
}

// SwapData struct.
type SwapData struct {
	JobID             JobID
	Attempt           int // Not used yet
	BoltzID           string
	State             State
	AllowZeroConf     bool
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

// ParseJobData gets a new JobData from bytes
func ParseJobData(id int32, bytes []byte) (*JobData, error) {
	var jd JobData
	err := json.Unmarshal(bytes, &jd)
	if err != nil {
		return nil, ErrCouldNotParseJobData
	}

	jd.ID = id
	switch jd.Target {
	case OutboundLiquidityNodePercent:
		fallthrough
	case InboundLiquidityNodePercent:
		if jd.Percentage <= 0 || jd.Percentage >= 100 {
			return nil, ErrCouldNotParseJobData
		}
		return &jd, nil
	case InboundLiquidityChannelPercent:
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
