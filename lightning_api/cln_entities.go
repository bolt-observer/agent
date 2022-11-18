package lightning_api

import (
	"errors"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

var (
	ErrNoNode          = errors.New("node not found")
	ErrNoChan          = errors.New("channel not found")
	ErrInvalidResponse = errors.New("invalid response")
)

type ClnInfo struct {
	PubKey    string            `json:"id"`
	Alias     string            `json:"alias"`
	Color     string            `json:"color"`
	Network   string            `json:"network"`
	Addresses []ClnListNodeAddr `json:"address,omitempty"`
	Features  ClnFeatures       `json:"our_features"`
}

type ClnFeatures struct {
	Init    string `json:"init"`
	Node    string `json:"node"`
	Channel string `json:"channe"`
	Invoice string `json:"invoice"`
}

type ClnSetChan struct {
	PeerId         string `json:"peer_id"`
	LongChanId     string `json:"channel_id"`
	FeeBase        string `json:"fee_base_msat"`
	FeePpm         string `json:"fee_proportional_milli"`
	MiHtlc         string `json:"minimum_htlc_out_msat"`
	MaxHtlc        string `json:"maximum_htlc_out_msat"`
	ShortChannelId string `json:"short_channel_id,omitempty"`
}

type ClnSetChanResp struct {
	Settings []ClnSetChan `json:"channels,omitempty"`
}

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
	ShortChannelId string            `json:"short_channel_id,omitempty"`
	Delay          uint64            `json:"delay"`
}

type ClnListChanResp struct {
	Channels []ClnListChan `json:"channels,omitempty"`
}

type ClnFundsChan struct {
	PeerId         string `json:"peer_id"`
	Connected      bool   `json:"connected,omitempty"`
	ShortChannelId string `json:"short_channel_id"`
	State          string `json:"state"`
	Capacity       uint64 `json:"channel_total_sat"`
	OurAmount      uint64 `json:"channel_sat"`
	FundingTxId    string `json:"funding_tx_id"`
	FundingOutput  int    `json:"funding_output"`
}

type ClnFundsChanResp struct {
	Channels []ClnFundsChan `json:"channels,omitempty"`
}

type ClnSocketLightningApi struct {
	LightningApi
	Client  *rpc.Client
	Timeout time.Duration
}

type ClnListNodeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type ClnListNode struct {
	PubKey     string             `json:"nodeid"`
	Alias      string             `json:"alias,omitempty"`
	Color      string             `json:"color,omitempty"`
	Features   string             `json:"features,omitempty"`
	Addresses  []ClnListNodeAddr  `json:"addresses,omitempty"`
	LastUpdate *entities.JsonTime `json:"last_update,omitempty"`
}

type ClnListNodeResp struct {
	Nodes []ClnListNode `json:"nodes,omitempty"`
}
