package entities

import (
	api "github.com/bolt-observer/agent/lightning"
)

// NodeDetails struct
type NodeDetails struct {
	NodeVersion                    string `json:"node_version"`
	IsSyncedToChain                bool   `json:"is_synced_to_chain"`
	IsSyncedToGraph                bool   `json:"is_synced_to_graph"`
	AgentVersion                   string `json:"agent_version"` // will be filled before sending
	TotalOnChainBalanceConfirmed   uint64 `json:"total_onchain_balance_confirmed"`
	TotalOnChainBalanceUnconfirmed uint64 `json:"total_onchain_balance_unconfirmed"`

	api.NodeInfoAPIExtended
}
