package entities

import (
	api "github.com/bolt-observer/agent/lightning"
)

// NodeDetails struct
type NodeDetails struct {
	NodeVersion   string `json:"node_version"`
	IsSyncedToChain bool   `json:"is_synced_to_chain"`
	IsSyncedToGraph bool   `json:"is_synced_to_graph"`

	api.NodeInfoAPIExtended
}
