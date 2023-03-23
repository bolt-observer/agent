package lightning

import (
	"encoding/json"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

// A hack to override uint64 -> string (REST API)

// GetInfoResponseOverride struct.
type GetInfoResponseOverride struct {
	BestHeaderTimestamp string `json:"best_header_timestamp,omitempty"`
	lnrpc.GetInfoResponse
}

// Channels struct.
type Channels struct {
	Channels []*ChannelOverride `json:"channels"`
}

// ChannelConstraintsOverride struct.
type ChannelConstraintsOverride struct {
	ChanReserveSat    string `json:"chan_reserve_sat,omitempty"`
	DustLimitSat      string `json:"dust_limit_sat,omitempty"`
	MaxPendingAmtMsat string `json:"max_pending_amt_msat,omitempty"`
	MinHtlcMsat       string `json:"min_htlc_msat,omitempty"`

	lnrpc.ChannelConstraints
}

// HtlcOverride struct.
type HtlcOverride struct {
	Amount              string `json:"amount,omitempty"`
	HashLock            string `json:"hash_lock,omitempty"`
	HtlcIndex           string `json:"htlc_index,omitempty"`
	ForwardingChannel   string `json:"forwarding_channel,omitempty"`
	ForwardingHtlcIndex string `json:"forwarding_htlc_index,omitempty"`
	lnrpc.HTLC
}

// ChannelOverride struct.
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

// Graph struct.
type Graph struct {
	GraphNodeOverride  []*GraphNodeOverride `json:"nodes,omitempty"`
	GraphEdgesOverride []*GraphEdgeOverride `json:"edges,omitempty"`
}

// RoutingPolicyOverride struct.
type RoutingPolicyOverride struct {
	MinHtlc          string `json:"min_htlc,omitempty"`
	FeeBaseMsat      string `json:"fee_base_msat,omitempty"`
	FeeRateMilliMsat string `json:"fee_rate_milli_msat,omitempty"`
	MaxHtlcMsat      string `json:"max_htlc_msat,omitempty"`

	lnrpc.RoutingPolicy
}

// GraphNodeOverride struct.
type GraphNodeOverride struct {
	lnrpc.LightningNode
}

// GetNodeInfoOverride struct.
type GetNodeInfoOverride struct {
	Node          *GraphNodeOverride   `json:"node,omitempty"`
	NumChannels   int64                `json:"num_channels,omitempty"`
	TotalCapacity string               `json:"total_capacity,omitempty"`
	Channels      []*GraphEdgeOverride `json:"channels"`

	lnrpc.NodeInfo
}

// GraphEdgeOverride struct.
type GraphEdgeOverride struct {
	ChannelID   string                 `json:"channel_id,omitempty"`
	Capacity    string                 `json:"capacity,omitempty"`
	Node1Policy *RoutingPolicyOverride `json:"node1_policy,omitempty"`
	Node2Policy *RoutingPolicyOverride `json:"node2_policy,omitempty"`

	lnrpc.ChannelEdge
}

// ForwardingHistoryRequestOverride struct.
type ForwardingHistoryRequestOverride struct {
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`

	lnrpc.ForwardingHistoryRequest
}

// ForwardingHistoryResponseOverride struct.
type ForwardingHistoryResponseOverride struct {
	ForwardingEvents []*ForwardingEventOverride `json:"forwarding_events,omitempty"`
	lnrpc.ForwardingHistoryResponse
}

// ForwardingEventOverride struct.
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

// HtlcEventOverride struct.
type HtlcEventOverride struct {
	IncomingChannelID string             `json:"incoming_channel_id,omitempty"`
	OutgoingChannelID string             `json:"outgoing_channel_id,omitempty"`
	IncomingHtlcID    string             `json:"incoming_htlc_id,omitempty"`
	OutgoingHtlcID    string             `json:"outgoing_htlc_id,omitempty"`
	TimestampNs       string             `json:"timestamp_ns,omitempty"`
	EventType         string             `json:"event_type,omitempty"`
	Event             HtlcEventComposite `json:"event,omitempty"`

	routerrpc.HtlcEvent
}

// HtlcInfoOverride struct.
type HtlcInfoOverride struct {
	IncomingAmtMsat string `json:"incoming_amt_msat,omitempty"`
	OutgoingAmtMsat string `json:"outgoing_amt_msat,omitempty"`
}

// HtlcEventComposite struct (our abstraction over that mess with inheritance).
type HtlcEventComposite struct {
	Info          *HtlcInfoOverride `json:"info,omitempty"`
	Preimage      string            `json:"preimage,omitempty"`
	WireFailure   string            `json:"wire_failure,omitempty"`
	FailureDetail string            `json:"failure_detail,omitempty"`
	FailureString string            `json:"failure_string,omitempty"`
}

// ListInvoiceRequestOverride struct.
type ListInvoiceRequestOverride struct {
	IndexOffset    string `json:"index_offset,omitempty"`
	NumMaxInvoices string `json:"num_max_invoices,omitempty"`

	lnrpc.ListInvoiceRequest
}

// ListInvoiceResponseOverride struct.
type ListInvoiceResponseOverride struct {
	Invoices         []*InvoiceOverride `json:"invoices,omitempty"`
	LastIndexOffset  string             `json:"last_index_offset,omitempty"`
	FirstIndexOffset string             `json:"first_index_offset,omitempty"`

	lnrpc.ListInvoiceResponse
}

// InvoiceOverride struct.
type InvoiceOverride struct {
	Value        string `json:"value,omitempty"`
	ValueMsat    string `json:"value_msat,omitempty"`
	RPreimage    string `json:"r_preimage,omitempty"`
	RHash        string `json:"r_hash,omitempty"`
	CreationDate string `json:"creation_date,omitempty"`
	SettleDate   string `json:"settle_date,omitempty"`
	Expiry       string `json:"expiry,omitempty"`
	CltvExpiry   string `json:"cltv_expiry,omitempty"`

	AddIndex    string `json:"add_index,omitempty"`
	SettleIndex string `json:"settle_index,omitempty"`
	AmtPaid     string `json:"amt_paid,omitempty"`
	AmtPaidSat  string `json:"amt_paid_sat,omitempty"`
	AmtPaidMsat string `json:"amt_paid_msat,omitempty"`

	DescriptionHash string               `json:"description_hash,omitempty"`
	RouteHints      []*RouteHintOverride `json:"route_hints,omitempty"`
	State           string               `json:"state,omitempty"`

	// Ignore this stuff
	Features        json.RawMessage `json:"features,omitempty"`
	Htlcs           json.RawMessage `json:"htlcs,omitempty"`
	AmpInvoiceState json.RawMessage `json:"amp_invoice_state,omitempty"`

	lnrpc.Invoice
}

// RouteHintOverride struct.
type RouteHintOverride struct {
	HopHints []*HopHintOverride `json:"hop_hints,omitempty"`

	lnrpc.RouteHint
}

// HopHintOverride struct.
type HopHintOverride struct {
	ChanID string `json:"chan_id,omitempty"`

	lnrpc.HopHint
}

// ListPaymentsRequestOverride struct.
type ListPaymentsRequestOverride struct {
	IndexOffset string `json:"index_offset,omitempty"`
	MaxPayments string `json:"max_payments,omitempty"`

	lnrpc.ListPaymentsRequest
}

// ListPaymentsResponseOverride struct.
type ListPaymentsResponseOverride struct {
	Payments         []*PaymentOverride `json:"payments,omitempty"`
	LastIndexOffset  string             `json:"last_index_offset,omitempty"`
	FirstIndexOffset string             `json:"first_index_offset,omitempty"`
	TotalNumPayments string             `json:"total_num_payments,omitempty"`

	lnrpc.ListPaymentsResponse
}

// PaymentOverride struct.
type PaymentOverride struct {
	Value          string                 `json:"value,omitempty"`
	CreationDate   string                 `json:"creation_date,omitempty"`
	Fee            string                 `json:"fee,omitempty"`
	ValueSat       string                 `json:"value_sat,omitempty"`
	ValueMsat      string                 `json:"value_msat,omitempty"`
	Status         string                 `json:"status,omitempty"`
	FeeSat         string                 `json:"fee_sat,omitempty"`
	FeeMsat        string                 `json:"fee_msat,omitempty"`
	CreationTimeNs string                 `json:"creation_time_ns,omitempty"`
	Htlcs          []*HTLCAttemptOverride `json:"htlcs,omitempty"`
	PaymentIndex   string                 `json:"payment_index,omitempty"`
	FailureReason  string                 `json:"failure_reason,omitempty"`

	lnrpc.Payment
}

// HTLCAttemptOverride struct.
type HTLCAttemptOverride struct {
	AttemptID     string          `json:"attempt_id,omitempty"`
	Status        string          `json:"status,omitempty"`
	Route         *RouteOverride  `json:"route,omitempty"`
	AttemptTimeNs string          `json:"attempt_time_ns,omitempty"`
	ResolveTimeNs string          `json:"resolve_time_ns,omitempty"`
	Failure       json.RawMessage `json:"failure,omitempty"` // ignore
	Preimage      string          `protobuf:"bytes,6,opt,name=preimage,proto3" json:"preimage,omitempty"`

	lnrpc.HTLCAttempt
}

// RouteOverride struct.
type RouteOverride struct {
	TotalFees     string         `json:"total_fees,omitempty"`
	TotalAmt      string         `json:"total_amt,omitempty"`
	Hops          []*HopOverride `json:"hops,omitempty"`
	TotalFeesMsat string         `json:"total_fees_msat,omitempty"`
	TotalAmtMsat  string         `json:"total_amt_msat,omitempty"`

	lnrpc.Route
}

// HopOverride struct.
type HopOverride struct {
	ChanID           string          `json:"chan_id,omitempty"`
	ChanCapacity     string          `json:"chan_capacity,omitempty"`
	AmtToForward     string          `json:"amt_to_forward,omitempty"`
	Fee              string          `json:"fee,omitempty"`
	AmtToForwardMsat string          `json:"amt_to_forward_msat,omitempty"`
	FeeMsat          string          `json:"fee_msat,omitempty"`
	MppRecord        json.RawMessage `json:"mpp_record,omitempty"`     // ignore
	AmpRecord        json.RawMessage `json:"amp_record,omitempty"`     // ignore
	CustomRecords    json.RawMessage `json:"custom_records,omitempty"` // ignore
	Metadata         string          `json:"metadata,omitempty"`

	lnrpc.Hop
}

// ConnectPeerRequestOverride struct.
type ConnectPeerRequestOverride struct {
	Addr    *LightningAddressOverride `json:"addr,omitempty"`
	Timeout string                    `json:"timeout,omitempty"`
	lnrpc.ConnectPeerRequest
}

// LightningAddressOverride struct.
type LightningAddressOverride struct {
	lnrpc.LightningAddress
}

// WalletBalanceResponseOverride struct.
type WalletBalanceResponseOverride struct {
	TotalBalance              string          `json:"total_balance,omitempty"`
	ConfirmedBalance          string          `json:"confirmed_balance,omitempty"`
	UnconfirmedBalance        string          `json:"unconfirmed_balance,omitempty"`
	LockedBalance             string          `json:"locked_balance,omitempty"`
	ReservedBalanceAnchorChan string          `json:"reserved_balance_anchor_chan,omitempty"`
	AccountBalance            json.RawMessage `json:"account_balance,omitempty"`

	lnrpc.WalletBalanceResponse
}

/*
// AccountBalanceEntryOverride struct.
type AccountBalanceEntryOverride struct {
	// ignore
}
*/

// SendCoinsRequestOverride struct.
type SendCoinsRequestOverride struct {
	Amount      string `json:"amount,omitempty"`
	SatPerVbyte string `json:"sat_per_vbyte,omitempty"`
	// Deprecated
	SatPerByte string `json:"sat_per_byte,omitempty"`
	lnrpc.SendCoinsRequest
}

// SendPaymentRequestOverride struct.
type SendPaymentRequestOverride struct {
	Dest         string `json:"dest,omitempty"`
	Amt          string `json:"amt,omitempty"`
	AmtMsat      string `json:"amt_msat,omitempty"`
	PaymentHash  string `json:"payment_hash,omitempty"`
	PaymentAddr  string `json:"payment_addr,omitempty"`
	FeeLimitSat  string `json:"fee_limit_sat,omitempty"`
	FeeLimitMsat string `json:"fee_limit_msat,omitempty"`
	// Deprecated: Do not use.
	OutgoingChanId    string                         `json:"outgoing_chan_id,omitempty"`
	OutgoingChanIds   []string                       `json:"outgoing_chan_ids,omitempty"`
	LastHopPubkey     string                         `json:"last_hop_pubkey,omitempty"`
	RouteHints        []RouteHintOverride            `json:"route_hints,omitempty"`
	DestCustomRecords DestCustomRecordsEntryOverride `json:"dest_custom_records,omitempty"`
	DestFeatures      []lnrpc.FeatureBit             `json:"dest_features,omitempty"`
	MaxShardSizeMsat  string                         `json:"max_shard_size_msat,omitempty"`

	routerrpc.SendPaymentRequest
}

// DestCustomRecordsEntryOverride struct.
type DestCustomRecordsEntryOverride struct {
	// ignore
}

// AddInvoiceResponseOverride struct.
type AddInvoiceResponseOverride struct {
	RHash       string `json:"r_hash,omitempty"`
	AddIndex    string `json:"add_index,omitempty"`
	PaymentAddr string `json:"payment_addr,omitempty"`

	lnrpc.AddInvoiceResponse
}

// TrackPaymentRequestOverride struct.
type TrackPaymentRequestOverride struct {
	PaymentHash string `json:"payment_hash,omitempty"`
	routerrpc.TrackPaymentRequest
}
