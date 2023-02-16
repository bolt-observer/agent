package entities

import (
	"context"

	"github.com/bolt-observer/go_common/entities"
)

// NodeDataReportCallback represents the nodedata callback.
type NodeDataReportCallback func(ctx context.Context, report *NodeDataReport) bool

// NodeDataReport struct.
type NodeDataReport struct {
	ReportingSettings
	// Chain - should be bitcoin (bitcoin, litecoin)
	Chain string `json:"chain"`
	// Network - should be mainnet (regtest, testnet, mainnet)
	Network string `json:"network"`
	// Pubkey - node pubkey
	PubKey string `json:"pubkey"`
	// UniqueID is the optional unique identifier
	UniqueID string `json:"uniqueId,omitempty"`
	// Timestamp - timestamp of report
	Timestamp entities.JsonTime `json:"timestamp"`
	// ChangedChannels - contains all channels were balance has changed
	ChangedChannels []ChannelBalance `json:"changed_channels"`
	// ClosedChannels - contains all channels that were determined to be closed
	ClosedChannels []ClosedChannel `json:"closed_channels"`
	// Node Details
	NodeDetails *NodeDetails `json:"node_details,omitempty"`
}
