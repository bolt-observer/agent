package entities

import api "github.com/bolt-observer/agent/lightning_api"

type NodeIdentifier struct {
	Identifier string `json:"identifier"`
	UniqueId   string `json:"unique_id"`
}

func (n *NodeIdentifier) GetId() string {
	return n.Identifier + n.UniqueId
}

type NewApiCall func() api.LightingApiCalls
