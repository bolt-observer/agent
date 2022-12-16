package entities

import (
	api "github.com/bolt-observer/agent/lightning_api"
	entities "github.com/bolt-observer/go_common/entities"
)

type NodeDataReport struct {
	ReportingSettings
	Chain           string            `json:"chain"`   // should be bitcoin (bitcoin, litecoin)
	Network         string            `json:"network"` // should be mainnet (regtest, testnet, mainnet)
	PubKey          string            `json:"pubkey"`
	UniqueId        string            `json:"uniqueId,omitempty"` // optional unique identifier
	Timestamp       entities.JsonTime `json:"timestamp"`
	ChangedChannels []ChannelBalance  `json:"changed_channels"`
	ClosedChannels  []ClosedChannel   `json:"closed_channels"`

	api.NodeInfoApiExtended
}
