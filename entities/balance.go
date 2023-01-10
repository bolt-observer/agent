package entities

import (
	"fmt"
	"strings"
	"time"

	"github.com/bolt-observer/agent/filter"
)

// Interval represents the enum of possible intervals
type Interval int

const (
	// ManualRequest means just do it once
	ManualRequest Interval = iota
	// Second means once per second (useful just for testing)
	Second
	// TenSeconds means once every ten seconds
	TenSeconds
	// Minute means once every minute
	Minute
	// TenMinutes means once every ten minutes
	TenMinutes
	// Hour means once every hour
	Hour
)

// Duration converts the interval to a duration
func (i Interval) Duration() time.Duration {
	switch i {
	case Second:
		return 1 * time.Second
	case TenSeconds:
		return 10 * time.Second
	case Minute:
		return 1 * time.Minute
	case TenMinutes:
		return 10 * time.Minute
	case Hour:
		return 1 * time.Hour
	default:
		return 0 * time.Second
	}
}

// MarshalJSON is used for JSON serialiazation of interval
func (i *Interval) MarshalJSON() ([]byte, error) {
	str := ""
	switch *i {
	case ManualRequest:
		str = `"manual"`
	case Second:
		str = `"1s"`
	case TenSeconds:
		str = `"10s"`
	case TenMinutes:
		str = `"10m"`
	case Minute:
		str = `"1m"`
	case Hour:
		str = `"1h"`
	default:
		return nil, fmt.Errorf("could not marshal interval: %v", i)
	}
	return []byte(str), nil
}

// UnmarshalJSON is used for JSON deserialiazation of interval
func (i *Interval) UnmarshalJSON(s []byte) (err error) {
	input := strings.ReplaceAll(strings.ToLower(string(s)), "\"", "")

	switch input {
	case "manual":
		*i = ManualRequest
	case "1s":
		*i = Second
	case "10s":
		*i = TenSeconds
	case "1m":
		*i = Minute
	case "10m":
		*i = TenMinutes
	case "1h":
		*i = Hour
	default:
		return fmt.Errorf("could not marshal interval: %v", i)
	}

	return nil
}

// ReportingSettings struct
type ReportingSettings struct {
	// GraphPollInterval - intervl for graph polling
	GraphPollInterval time.Duration `json:"-"`
	// NoopInterval - send keepalive also when no changes happened
	NoopInterval time.Duration `json:"-"`
	// Filter is used to filter specific channels
	Filter filter.FilteringInterface `json:"-"`
	// PollInterval - defines how often the polling happens
	PollInterval Interval `json:"poll_interval"`
	// AllowedEntropy - is user specified entropy that can be reported
	AllowedEntropy int `json:"allowed_entropy"` // 64 bits is the default
	// AllowPrivateChannels - whether private channels were allowed (fitering will set this to true too)
	AllowPrivateChannels bool `json:"allow_private_channels"` // default is false
}

// ClosedChannel struct
type ClosedChannel struct {
	ChannelID uint64 `json:"channel_id"`
}

// ChannelBalance struct
type ChannelBalance struct {
	// Active - is channel active
	Active bool `json:"active"`
	// Private - is channel private
	Private bool `json:"private"`
	// Deprecated - ActivePrevious is whether channel was Active in previous run
	ActivePrevious bool `json:"active_previous"`

	// LocalPubkey is the local node pubkey
	LocalPubkey string `json:"local_pubkey"`
	// RemotePubkey is the remote node pubkey
	RemotePubkey string `json:"remote_pubkey"`
	// ChanID - short channel id
	ChanID uint64 `json:"chan_id"`
	// Capacity - capacity of channel in satoshis
	Capacity uint64 `json:"capacity"`

	// RemoteNominator - is the remote side nominator
	RemoteNominator uint64 `json:"remote_nominator"`
	// LocalNominator - is the local side nominator
	LocalNominator uint64 `json:"local_nominator"`
	// Denominator - is the denominator (with high entropy allowed this is more or less channnel capacity)
	Denominator uint64 `json:"denominator"`

	// Deprecated - RemoteNominatorDiff is the difference of RemoteNominator between now and previous run
	RemoteNominatorDiff int64 `json:"remote_nominator_diff"`
	// Deprecated - LocalNominatorDiff is the difference of LocalNominator between now and previous run
	LocalNominatorDiff int64 `json:"local_nominator_diff"`
	// Deprecated - DenominatorDiff is the difference of Denominator between now and previous run
	DenominatorDiff int64 `json:"denominator_diff"`

	// ActiveRemote - does other node specify channel as active
	ActiveRemote bool `json:"active_remote"`
	// Deprecated - ActiveRemotePrevious - ActiveRemote from previous run
	ActiveRemotePrevious bool `json:"active_remote_previous"`
	// ActiveLocal - does current node specify channel as active
	ActiveLocal bool `json:"active_local"`
	// Deprecated - ActiveLocalPrevious - ActiveLocal from previous run
	ActiveLocalPrevious bool `json:"active_local_previous"`
}
