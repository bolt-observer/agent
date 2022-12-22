package lightningApi

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

// ClnInfo
type ClnInfo struct {
	PubKey    string            `json:"id"`
	Alias     string            `json:"alias"`
	Color     string            `json:"color"`
	Network   string            `json:"network"`
	Addresses []ClnListNodeAddr `json:"address,omitempty"`
	Features  ClnFeatures       `json:"our_features"`
}

// ClnFeatures
type ClnFeatures struct {
	Init    string `json:"init"`
	Node    string `json:"node"`
	Channel string `json:"channe"`
	Invoice string `json:"invoice"`
}

// ClnSetChan
type ClnSetChan struct {
	PeerID         string `json:"peer_id"`
	LongChanID     string `json:"channel_id"`
	FeeBase        string `json:"fee_base_msat"`
	FeePpm         string `json:"fee_proportional_milli"`
	MiHtlc         string `json:"minimum_htlc_out_msat"`
	MaxHtlc        string `json:"maximum_htlc_out_msat"`
	ShortChannelID string `json:"short_channel_id,omitempty"`
}

// ClnSetChanResp
type ClnSetChanResp struct {
	Settings []ClnSetChan `json:"channels,omitempty"`
}

// ClnListChan
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

// ClnListChanResp
type ClnListChanResp struct {
	Channels []ClnListChan `json:"channels,omitempty"`
}

// ClnFundsChan
type ClnFundsChan struct {
	PeerId         string `json:"peer_id"`
	Connected      bool   `json:"connected,omitempty"`
	ShortChannelID string `json:"short_channel_id"`
	State          string `json:"state"`
	Capacity       uint64 `json:"channel_total_sat"`
	OurAmount      uint64 `json:"channel_sat"`
	FundingTxID    string `json:"funding_tx_id"`
	FundingOutput  int    `json:"funding_output"`
}

// ClnFundsChanResp
type ClnFundsChanResp struct {
	Channels []ClnFundsChan `json:"channels,omitempty"`
}

// ClnSocketLightningApi
type ClnSocketLightningApi struct {
	LightningApi
	Client  *rpc.Client
	Timeout time.Duration
}

// ClnListNodeAddr
type ClnListNodeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ClnListNode
type ClnListNode struct {
	PubKey     string             `json:"nodeid"`
	Alias      string             `json:"alias,omitempty"`
	Color      string             `json:"color,omitempty"`
	Features   string             `json:"features,omitempty"`
	Addresses  []ClnListNodeAddr  `json:"addresses,omitempty"`
	LastUpdate *entities.JsonTime `json:"last_update,omitempty"`
}

// ClnListNodeResp
type ClnListNodeResp struct {
	Nodes []ClnListNode `json:"nodes,omitempty"`
}
