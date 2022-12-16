package entities

type NodeDataReport struct {
	NodeReport  *InfoReport           `json:"node_report"`
	AgentReport *ChannelBalanceReport `json:"agent_report"`
}
