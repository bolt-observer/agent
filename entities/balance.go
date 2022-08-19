package entities

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type NodeIdentifier struct {
	Identifier string `json:"identifier"`
	UniqueId   string `json:"unique_id"`
}

type Interval int

const (
	MANUAL_REQUEST Interval = iota
	SECOND
	TEN_SECONDS
	MINUTE
	HOUR
)

func (i Interval) Duration() time.Duration {
	switch i {
	case SECOND:
		return 1 * time.Second
	case TEN_SECONDS:
		return 10 * time.Second
	case MINUTE:
		return 1 * time.Minute
	case HOUR:
		return 1 * time.Hour
	default:
		return 0 * time.Second
	}
}

func (i *Interval) MarshalJSON() ([]byte, error) {
	str := ""
	switch *i {
	case MANUAL_REQUEST:
		str = `"manual"`
	case SECOND:
		str = `"1s"`
	case TEN_SECONDS:
		str = `"10s"`
	case MINUTE:
		str = `"1m"`
	case HOUR:
		str = `"1h"`
	default:
		return nil, fmt.Errorf("could not marshal interval: %v", i)
	}
	return []byte(str), nil
}

func (i *Interval) UnmarshalJSON(s []byte) (err error) {
	input := strings.ReplaceAll(strings.ToLower(string(s)), "\"", "")

	switch input {
	case "manual":
		*i = MANUAL_REQUEST
	case "1s":
		*i = SECOND
	case "10s":
		*i = TEN_SECONDS
	case "1m":
		*i = MINUTE
	case "1h":
		*i = HOUR
	default:
		return fmt.Errorf("could not marshal interval: %v", i)
	}

	return nil
}

type BalanceReportCallback func(ctx context.Context, report *ChannelBalanceReport) bool

type JsonTime time.Time

func (t JsonTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(time.Time(t).Unix(), 10)), nil
}

func (t *JsonTime) UnmarshalJSON(s []byte) (err error) {
	r := strings.Replace(string(s), `"`, ``, -1)

	q, err := strconv.ParseInt(r, 10, 64)
	if err != nil {
		return err
	}
	*(*time.Time)(t) = time.Unix(q, 0)
	return
}

type ReportingSettings struct {
	GraphPollInterval    time.Duration `json:"-"`
	NoopInterval         time.Duration `json:"-"` // If that much time has passed send null report
	PollInterval         Interval      `json:"poll_interval"`
	AllowedEntropy       int           `json:"allowed_entropy"`        // 64 bits is the default
	AllowPrivateChannels bool          `json:"allow_private_channels"` // default is false
}

type ChannelBalanceReport struct {
	ReportingSettings
	Chain           string           `json:"chain"`   // should be bitcoin (bitcoin, litecoin)
	Network         string           `json:"network"` // should be mainnet (regtest, testnet, mainnet)
	PubKey          string           `json:"pubkey"`
	Timestamp       JsonTime         `json:"timestamp"`
	ChangedChannels []ChannelBalance `json:"changed_channels"`
	ClosedChannels  []ClosedChannel  `json:"closed_channels"`
}

type ClosedChannel struct {
	ChannelId uint64 `json:"channel_id"`
}

type ChannelBalance struct {
	Active  bool `json:"active"`
	Private bool `json:"private"`
	// Deprecated
	ActivePrevious bool `json:"active_previous"`

	LocalPubkey  string `json:"local_pubkey"`
	RemotePubkey string `json:"remote_pubkey"`
	ChanId       uint64 `json:"chan_id"`
	Capacity     uint64 `json:"capacity"`

	Nominator   uint64 `json:"nominator"`
	Denominator uint64 `json:"denominator"`

	// Deprecated
	NominatorDiff int64 `json:"nominator_diff"`
	// Deprecated
	DenominatorDiff int64 `json:"denominator_diff"`

	ActiveRemote bool `json:"active_remote"`
	// Deprecated
	ActiveRemotePrevious bool `json:"active_remote_previous"`
	ActiveLocal          bool `json:"active_local"`
	// Deprecated
	ActiveLocalPrevious bool `json:"active_local_previous"`
}
