package entities

import "context"

type NodeDataReportCallback func(ctx context.Context, report *NodeDataReport) bool

type NodeDataReport struct {
	NodeReport  	*InfoReport           `json:"node_report"`
	ChannelReport *ChannelBalanceReport `json:"channel_report"`
}
