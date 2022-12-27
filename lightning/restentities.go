package lightning

import "github.com/lightningnetwork/lnd/lnrpc"

// A hack to override uint64 -> string (REST API)

// GetInfoResponseOverride struct
type GetInfoResponseOverride struct {
	BestHeaderTimestamp string `json:"best_header_timestamp,omitempty"`
	lnrpc.GetInfoResponse
}

// Channels struct
type Channels struct {
	Channels []*ChannelOverride `json:"channels"`
}

// ChannelConstraintsOverride struct
type ChannelConstraintsOverride struct {
	ChanReserveSat    string `json:"chan_reserve_sat,omitempty"`
	DustLimitSat      string `json:"dust_limit_sat,omitempty"`
	MaxPendingAmtMsat string `json:"max_pending_amt_msat,omitempty"`
	MinHtlcMsat       string `json:"min_htlc_msat,omitempty"`

	lnrpc.ChannelConstraints
}

// HtlcOverride struct
type HtlcOverride struct {
	Amount              string `json:"amount,omitempty"`
	HashLock            string `json:"hash_lock,omitempty"`
	HtlcIndex           string `json:"htlc_index,omitempty"`
	ForwardingChannel   string `json:"forwarding_channel,omitempty"`
	ForwardingHtlcIndex string `json:"forwarding_htlc_index,omitempty"`
	lnrpc.HTLC
}

// ChannelOverride struct
type ChannelOverride struct {
	ChanID                string `json:"chan_id,omitempty"`
	Capacity              string `json:"capacity,omitempty"`
	LocalBalance          string `json:"local_balance,omitempty"`
	RemoteBalance         string `json:"remote_balance,omitempty"`
	CommitFee             string `json:"commit_fee,omitempty"`
	CommitWeight          string `json:"commit_weight,omitempty"`
	FeePerKw              string `json:"fee_per_kw,omitempty"`
	UnsettledBalance      string `json:"unsettled_balance,omitempty"`
	TotalSatoshisSent     string `json:"total_satoshis_sent,omitempty"`
	TotalSatoshisReceived string `json:"total_satoshis_received,omitempty"`
	NumUpdates            string `json:"num_updates,omitempty"`
	// Deprecated
	LocalChanReserveSat string `json:"local_chan_reserve_sat,omitempty"`
	// Deprecated
	RemoteChanReserveSat string                      `json:"remote_chan_reserve_sat,omitempty"`
	CommitmentType       string                      `json:"commitment_type,omitempty"`
	PendingHtlcs         []*HtlcOverride             `json:"pending_htlcs,omitempty"`
	Lifetime             string                      `json:"lifetime,omitempty"`
	Uptime               string                      `json:"uptime,omitempty"`
	PushAmountSat        string                      `json:"push_amount_sat,omitempty"`
	LocalConstraints     *ChannelConstraintsOverride `json:"local_constraints,omitempty"`
	RemoteConstraints    *ChannelConstraintsOverride `json:"remote_constraints,omitempty"`

	AliasScids            []string `json:"alias_scids,omitempty"`
	ZeroConfConfirmedScid string   `json:"zero_conf_confirmed_scid,omitempty"`

	lnrpc.Channel
}

// Graph struct
type Graph struct {
	GraphNodeOverride  []*GraphNodeOverride `json:"nodes,omitempty"`
	GraphEdgesOverride []*GraphEdgeOverride `json:"edges,omitempty"`

	//lnrpc.ChannelGraph
}

// RoutingPolicyOverride struct
type RoutingPolicyOverride struct {
	MinHtlc          string `json:"min_htlc,omitempty"`
	FeeBaseMsat      string `json:"fee_base_msat,omitempty"`
	FeeRateMilliMsat string `json:"fee_rate_milli_msat,omitempty"`
	MaxHtlcMsat      string `json:"max_htlc_msat,omitempty"`

	lnrpc.RoutingPolicy
}

// GraphNodeOverride struct
type GraphNodeOverride struct {
	lnrpc.LightningNode
}

// GetNodeInfoOverride struct
type GetNodeInfoOverride struct {
	Node          *GraphNodeOverride   `json:"node,omitempty"`
	NumChannels   int64                `json:"num_channels,omitempty"`
	TotalCapacity string               `json:"total_capacity,omitempty"`
	Channels      []*GraphEdgeOverride `json:"channels"`

	lnrpc.NodeInfo
}

// GraphEdgeOverride struct
type GraphEdgeOverride struct {
	ChannelID   string                 `json:"channel_id,omitempty"`
	Capacity    string                 `json:"capacity,omitempty"`
	Node1Policy *RoutingPolicyOverride `json:"node1_policy,omitempty"`
	Node2Policy *RoutingPolicyOverride `json:"node2_policy,omitempty"`

	lnrpc.ChannelEdge
}

// ForwardingHistoryRequestOverride struct
type ForwardingHistoryRequestOverride struct {
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`

	lnrpc.ForwardingHistoryRequest
}

// ForwardingHistoryResponseOverride struct
type ForwardingHistoryResponseOverride struct {
	ForwardingEvents []*ForwardingEventOverride `json:"forwarding_events,omitempty"`
	lnrpc.ForwardingHistoryResponse
}

// ForwardingEventOverride struct
type ForwardingEventOverride struct {
	Timestamp   string `json:"timestamp,omitempty"`
	ChanIDIn    string `json:"chan_id_in,omitempty"`
	ChanIDOut   string `json:"chan_id_out,omitempty"`
	AmtIn       string `json:"amt_in,omitempty"`
	AmtOut      string `json:"amt_out,omitempty"`
	Fee         string `json:"fee,omitempty"`
	FeeMsat     string `json:"fee_msat,omitempty"`
	AmtInMsat   string `json:"amt_in_msat,omitempty"`
	AmtOutMsat  string `json:"amt_out_msat,omitempty"`
	TimestampNs string `json:"timestamp_ns,omitempty"`

	lnrpc.ForwardingEvent
}
