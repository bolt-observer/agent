package entities

import "context"

// NodeDataReportCallback represents the nodedata callback
type NodeDataReportCallback func(ctx context.Context, report *NodeDataReport) bool

// NodeDataReport struct
type NodeDataReport struct {
	NodeReport    *InfoReport           `json:"node_report"`
	ChannelReport *ChannelBalanceReport `json:"channel_report"`
}
