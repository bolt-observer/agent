package entities

import (
	api "github.com/bolt-observer/agent/lightning"
)

// NodeDetails struct
type NodeDetails struct {
	NodeVersion               string `json:"node_version"`
	IsSyncedToChain           bool   `json:"is_synced_to_chain"`
	IsSyncedToGraph           bool   `json:"is_synced_to_graph"`
	AgentVersion              string `json:"agent_version"`                          // will be filled before sending
	OnChainBalanceNotReported bool   `json:"onchain_balance_not_reported,omitempty"` // or else we cannot distinguish 0 and disallowed to report
	OnChainBalanceConfirmed   uint64 `json:"onchain_balance_confirmed"`
	OnChainBalanceTotal       uint64 `json:"onchain_balance_total"`

	api.NodeInfoAPIExtended
}
