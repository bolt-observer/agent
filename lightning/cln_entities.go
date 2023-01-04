package lightning

import (
	"errors"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

var (
	// ErrNoNode means node was not found
	ErrNoNode = errors.New("node not found")
	// ErrNoChan means channel was not found
	ErrNoChan = errors.New("channel not found")
	// ErrInvalidResponse indicates invaid response
	ErrInvalidResponse = errors.New("invalid response")
)

// ClnInfo struct
type ClnInfo struct {
	PubKey                string            `json:"id"`
	Alias                 string            `json:"alias"`
	Color                 string            `json:"color"`
	Network               string            `json:"network"`
	Addresses             []ClnListNodeAddr `json:"address,omitempty"`
	Features              ClnFeatures       `json:"our_features"`
	Version               string            `json:"version"`
	WarningBitcoindSync   string            `json:"warning_bitcoind_sync,omitempty"`
	WarningLightningdSync string            `json:"warning_lightningd_sync,omitempty"`
}

// ClnFeatures struct
type ClnFeatures struct {
	Init    string `json:"init"`
	Node    string `json:"node"`
	Channel string `json:"channe"`
	Invoice string `json:"invoice"`
}

// ClnSetChan struct
type ClnSetChan struct {
	PeerID         string `json:"peer_id"`
	LongChanID     string `json:"channel_id"`
	FeeBase        string `json:"fee_base_msat"`
	FeePpm         string `json:"fee_proportional_milli"`
	MiHtlc         string `json:"minimum_htlc_out_msat"`
	MaxHtlc        string `json:"maximum_htlc_out_msat"`
	ShortChannelID string `json:"short_channel_id,omitempty"`
}

// ClnSetChanResp struct
type ClnSetChanResp struct {
	Settings []ClnSetChan `json:"channels,omitempty"`
}

// ClnListChan struct
type ClnListChan struct {
	Source         string            `json:"source"`
	Destination    string            `json:"destination"`
	Public         bool              `json:"public"`
	Capacity       uint64            `json:"satoshis"`
	Active         bool              `json:"active"`
	LastUpdate     entities.JsonTime `json:"last_update"`
	FeeBase        uint64            `json:"base_fee_millisatoshi"`
	FeePpm         uint64            `json:"fee_per_millionth"`
	MinHtlc        string            `json:"htlc_minimum_msat"`
	MaxHtlc        string            `json:"htlc_maximum_msat"`
	ShortChannelID string            `json:"short_channel_id,omitempty"`
	Delay          uint64            `json:"delay"`
}

// ClnListChanResp struct
type ClnListChanResp struct {
	Channels []ClnListChan `json:"channels,omitempty"`
}

// ClnFundsChan struct
type ClnFundsChan struct {
	PeerID         string `json:"peer_id"`
	Connected      bool   `json:"connected,omitempty"`
	ShortChannelID string `json:"short_channel_id"`
	State          string `json:"state"`
	Capacity       uint64 `json:"channel_total_sat"`
	OurAmount      uint64 `json:"channel_sat"`
	FundingTxID    string `json:"funding_tx_id"`
	FundingOutput  int    `json:"funding_output"`
}

// ClnFundsChanResp struct
type ClnFundsChanResp struct {
	Channels []ClnFundsChan `json:"channels,omitempty"`
}

// ClnSocketLightningAPI struct
type ClnSocketLightningAPI struct {
	API
	Client  *rpc.Client
	Timeout time.Duration
}

// ClnListNodeAddr struct
type ClnListNodeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ClnListNode struct
type ClnListNode struct {
	PubKey     string             `json:"nodeid"`
	Alias      string             `json:"alias,omitempty"`
	Color      string             `json:"color,omitempty"`
	Features   string             `json:"features,omitempty"`
	Addresses  []ClnListNodeAddr  `json:"addresses,omitempty"`
	LastUpdate *entities.JsonTime `json:"last_update,omitempty"`
}

// ClnListNodeResp struct
type ClnListNodeResp struct {
	Nodes []ClnListNode `json:"nodes,omitempty"`
}

// ClnForwardEntry struct
type ClnForwardEntry struct {
	InChannel    string            `json:"in_channel"`
	InMsat       string            `json:"in_msat"`
	Status       string            `json:"status"` // one of "offered", "settled", "local_failed", "failed"
	ReceivedTime entities.JsonTime `json:"received_time"`

	OutChannel string `json:"out_channel,omitempty"`
	OutMsat    string `json:"out_msat,omitempty"`
	FeeMsat    string `json:"fee_msat,omitempty"`
	FailCode   uint32 `json:"fail_code,omitempty"`
	FailReason string `json:"fail_reason,omitempty"`
}

// ClnForwardEntries struct
type ClnForwardEntries struct {
	Entries []ClnForwardEntry
}
