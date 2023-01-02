package entities

import (
	api "github.com/bolt-observer/agent/lightning"
)

// NodeDetails struct
type NodeDetails struct {
	NodeVersion   string `json:"node_version"`
	SyncedToChain bool   `json:"synced_to_chain"`
	SyncedToGraph bool   `json:"synced_to_graph"`

	api.NodeInfoAPIExtended
}
