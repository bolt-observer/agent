package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
)

// APIType enum
type APIType int

// ApiTypes
const (
	LndGrpc APIType = iota
	LndRest
	ClnSocket
)

// GetAPIType from integer
func GetAPIType(t *int) (*APIType, error) {
	if t == nil {
		return nil, fmt.Errorf("no api type specified")
	}
	if *t != int(LndGrpc) && *t != int(LndRest) && *t != int(ClnSocket) {
		return nil, fmt.Errorf("invalid api type specified")
	}

	ret := APIType(*t)
	return &ret, nil
}

// InfoAPI struct
type InfoAPI struct {
	IdentityPubkey  string
	Alias           string
	Chain           string
	Network         string
	Version         string
	IsSyncedToGraph bool
	IsSyncedToChain bool
}

// ChannelsAPI struct
type ChannelsAPI struct {
	Channels []ChannelAPI
}

// ChannelAPI struct
type ChannelAPI struct {
	Private               bool
	Active                bool
	RemotePubkey          string
	Initiator             bool
	CommitFee             uint64
	ChanID                uint64
	RemoteBalance         uint64
	LocalBalance          uint64
	Capacity              uint64
	PendingHtlcs          []HtlcAPI
	TotalSatoshisSent     uint64
	TotalSatoshisReceived uint64
	NumUpdates            uint64
}

// HtlcAPI struct
type HtlcAPI struct {
	Amount              uint64
	Incoming            bool
	ForwardingChannel   uint64
	ForwardingHtlcIndex uint64
}

// DescribeGraphAPI struct
type DescribeGraphAPI struct {
	Nodes    []DescribeGraphNodeAPI
	Channels []NodeChannelAPI
}

// DescribeGraphNodeAPI struct
type DescribeGraphNodeAPI struct {
	PubKey     string                    `json:"pub_key,omitempty"`
	Alias      string                    `json:"alias,omitempty"`
	Color      string                    `json:"color,omitempty"`
	Addresses  []NodeAddressAPI          `json:"addresses,omitempty"`
	Features   map[string]NodeFeatureAPI `json:"features,omitempty"`
	LastUpdate entities.JsonTime         `json:"last_update,omitempty"`
}

// NodeAddressAPI struct
type NodeAddressAPI struct {
	Network string `json:"network,omitempty"`
	Addr    string `json:"addr,omitempty"`
}

// NodeFeatureAPI struct
type NodeFeatureAPI struct {
	Name       string `json:"name,omitempty"`
	IsRequired bool   `json:"is_required,omitempty"`
	IsKnown    bool   `json:"is_known,omitempty"`
}

// NodeChannelAPI struct
type NodeChannelAPI struct {
	ChannelID   uint64            `json:"channel_id,omitempty"`
	ChanPoint   string            `json:"chan_point"`
	Node1Pub    string            `json:"node1_pub,omitempty"`
	Node2Pub    string            `json:"node2_pub,omitempty"`
	Capacity    uint64            `json:"capacity,omitempty"`
	Node1Policy *RoutingPolicyAPI `json:"node1_policy,omitempty"`
	Node2Policy *RoutingPolicyAPI `json:"node2_policy,omitempty"`
	LastUpdate  entities.JsonTime `json:"last_update,omitempty"`
}

// RoutingPolicyAPI struct
type RoutingPolicyAPI struct {
	TimeLockDelta uint32            `json:"time_lock_delta"`
	MinHtlc       uint64            `json:"min_htlc"`
	BaseFee       uint64            `json:"fee_base_msat"`
	FeeRate       uint64            `json:"fee_rate_milli_msat"`
	Disabled      bool              `json:"disabled,omitempty"`
	LastUpdate    entities.JsonTime `json:"last_update,omitempty"`
	MaxHtlc       uint64            `json:"max_htlc_msat"`
}

// NodeInfoAPI struct
type NodeInfoAPI struct {
	Node          DescribeGraphNodeAPI `json:"node,omitempty"`
	Channels      []NodeChannelAPI     `json:"channels"`
	NumChannels   uint32               `json:"num_channels"`
	TotalCapacity uint64               `json:"total_capacity"`
}

// NodeChannelAPIExtended struct
type NodeChannelAPIExtended struct {
	Private bool `json:"private,omitempty"`
	NodeChannelAPI
}

// NodeInfoAPIExtended struct
type NodeInfoAPIExtended struct {
	NodeInfoAPI
	Channels []NodeChannelAPIExtended `json:"channels"`
}

////////////////////////////////////////////////////////////////

// Pagination struct
type Pagination struct {
	Offset   uint64 // Exclusive thus 1 means start from 2 (0 will start from beginning)
	Num      uint64 // limit is 10k or so
	Reversed bool
	From     *time.Time
	To       *time.Time
}

// PaymentStatus enum
type PaymentStatus int

// PaymentStatus values
const (
	PaymentUnknown PaymentStatus = 0
	PaymentInFlight
	PaymentSucceeded
	PaymentFailed
)

// StringToPaymentStatus creates PaymentStatus based on a string
func StringToPaymentStatus(in string) PaymentStatus {
	switch strings.ToLower(in) {
	case "unknown":
		return PaymentUnknown
	case "in_flight":
		return PaymentInFlight
	case "succeeded":
		return PaymentSucceeded
	case "failed":
		return PaymentFailed
	}

	return PaymentUnknown
}

// PaymentFailureReason enum
type PaymentFailureReason int

// PaymentFailureReason values
const (
	FailureReasonNone PaymentFailureReason = 0
	FailureReasonTimeout
	FailureReasonNoRoute
	FailureReasonError
	FailureReasonIncorrectPaymentDetails
	FailureReasonInsufficientBalance
)

// HTLCStatus enum
type HTLCStatus int

// HTLCStatus values
const (
	HTLCInFlight HTLCStatus = 0
	HTLCSucceeded
	HTLCFailed
)

// Payment struct
type Payment struct {
	PaymentHash     string
	ValueMsat       int64
	FeeMsat         int64
	PaymentPreimage string
	PaymentRequest  string
	PaymentStatus   PaymentStatus
	CreationTime    time.Time
	Index           uint64
	FailureReason   PaymentFailureReason
	HTLCAttempts    []HTLCAttempt
}

// HTLCAttempt struct
type HTLCAttempt struct {
	ID      uint64
	Status  HTLCStatus
	Attempt time.Time
	Resolve time.Time

	Route Route
}

// Route struct
type Route struct {
	TotalTimeLock uint32
	TotalFeesMsat int64
	TotalAmtMsat  int64

	Hops []Hop
}

// Hop struct
type Hop struct {
	ChanID           uint64
	Expiry           uint32
	AmtToForwardMsat int64
	FeeMsat          int64
}

// ForwardingEvent struct
type ForwardingEvent struct {
	Timestamp     time.Time
	ChanIDIn      uint64
	ChanIDOut     uint64
	AmountInMsat  uint64
	AmountOutMsat uint64
	FeeMsat       uint64
	IsSuccess     bool
	FailureString string
}

// ResponseForwardPagination struct
type ResponseForwardPagination struct {
	LastOffsetIndex uint64
}

// ResponsePagination struct
type ResponsePagination struct {
	ResponseForwardPagination
	FirstOffsetIndex uint64
}

// InvoicesResponse struct
type InvoicesResponse struct {
	Invoices []Invoice
	ResponsePagination
}

// Invoice struct
type Invoice struct {
	Memo            string
	ValueMsat       int64
	PaidMsat        int64
	CreationDate    time.Time
	SettleDate      time.Time
	PaymentRequest  string
	DescriptionHash string
	Expiry          int64
	FallbackAddr    string
	CltvExpiry      uint64
	Private         bool
	IsKeySend       bool
	IsAmp           bool
	State           InvoiceHTLCState
	AddIndex        uint64
	SettleIndex     uint64
}

// InvoiceHTLCState enum
type InvoiceHTLCState int

// StringToInvoiceHTLCState creates InvoiceHTLCState based on a string
func StringToInvoiceHTLCState(in string) InvoiceHTLCState {
	switch strings.ToLower(in) {
	case "accepted":
		return InvoiceAccepted
	case "settled":
		return InvoiceSettled
	case "cacelled":
		return InvoiceCancelled
	}

	return InvoiceCancelled
}

// InvoiceHTLCState values
const (
	InvoiceAccepted InvoiceHTLCState = 0
	InvoiceSettled
	InvoiceCancelled
)

// PaymentsResponse struct
type PaymentsResponse struct {
	Payments []Payment
	ResponsePagination
}

// RawMessage struct
type RawMessage struct {
	Timestamp      time.Time `json:"timestamp"`
	Implementation string    `json:"implementation,omitempty"`

	Message json.RawMessage `json:"message,omitempty"`
}

// ResponseRawPagination struct
type ResponseRawPagination struct {
	UseTimestamp bool
	FirstTime    time.Time
	LastTime     time.Time
	ResponsePagination
}

// RawPagination struct
type RawPagination struct {
	UseTimestamp bool
	FirstTime    time.Time
	LastTime     time.Time
	Pagination
}

////////////////////////////////////////////////////////////////

// API - generic API settings
type API struct {
	GetNodeInfoFullThreshUseDescribeGraph int // If node has more than that number of channels use DescribeGraph else do GetChanInfo for each one
}

// GetNodeInfoFull - GetNodeInfoFull API (GRPC interface)
func (l *LndGrpcLightningAPI) GetNodeInfoFull(ctx context.Context, channels, unnanounced bool) (*NodeInfoAPIExtended, error) {
	return getNodeInfoFullTemplate(ctx, l, l.GetNodeInfoFullThreshUseDescribeGraph, channels, unnanounced)
}

// GetNodeInfoFull - GetNodeInfoFull API (REST interface)
func (l *LndRestLightningAPI) GetNodeInfoFull(ctx context.Context, channels, unnanounced bool) (*NodeInfoAPIExtended, error) {
	return getNodeInfoFullTemplate(ctx, l, l.GetNodeInfoFullThreshUseDescribeGraph, channels, unnanounced)
}

// getNodeInfoFullTemplate returns info for local node possibly including unnanounced channels (as soon as that can be obtained via GetNodeInfo this method is useless)
func getNodeInfoFullTemplate(ctx context.Context, l LightingAPICalls, threshUseDescribeGraph int, channels, unnanounced bool) (*NodeInfoAPIExtended, error) {
	info, err := l.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := l.GetNodeInfo(ctx, info.IdentityPubkey, channels)
	if err != nil {
		return nil, err
	}

	extendedNodeInfo := &NodeInfoAPIExtended{NodeInfoAPI: *nodeInfo}

	if !unnanounced {
		// We have full info already (fast bailout)

		capacity := uint64(0)
		all := make([]NodeChannelAPIExtended, 0)
		for _, ch := range nodeInfo.Channels {
			all = append(all, NodeChannelAPIExtended{NodeChannelAPI: ch, Private: false})
			capacity += ch.Capacity
		}

		extendedNodeInfo.Channels = all
		extendedNodeInfo.NumChannels = uint32(len(all))
		extendedNodeInfo.TotalCapacity = capacity

		return extendedNodeInfo, err
	}

	// Else the channel stats are wrong (unnanounced channels did not count)
	chans, err := l.GetChannels(ctx)
	if err != nil {
		// TODO: Bit of a hack but nodeInfo is pretty much correct
		return extendedNodeInfo, err
	}

	numChans := 0
	totalCapacity := uint64(0)

	privateMapping := make(map[uint64]bool)

	for _, ch := range chans.Channels {
		if ch.Private && !unnanounced {
			continue
		}

		privateMapping[ch.ChanID] = ch.Private
		totalCapacity += ch.Capacity
		numChans++
	}

	extendedNodeInfo.NumChannels = uint32(numChans)
	extendedNodeInfo.TotalCapacity = totalCapacity

	if !channels {
		return extendedNodeInfo, nil
	}

	extendedNodeInfo.Channels = make([]NodeChannelAPIExtended, 0)

	if len(chans.Channels) <= threshUseDescribeGraph {
		for _, ch := range chans.Channels {
			if ch.Private && !unnanounced {
				continue
			}
			c, err := l.GetChanInfo(ctx, ch.ChanID)

			if err != nil {
				glog.Warningf("Could not get channel info for %v: %v", ch.ChanID, err)
				extendedNodeInfo.NumChannels--
				continue
			}
			private, ok := privateMapping[ch.ChanID]
			extendedNodeInfo.Channels = append(extendedNodeInfo.Channels, NodeChannelAPIExtended{NodeChannelAPI: *c, Private: ok && private})
		}
	} else {
		graph, err := l.DescribeGraph(ctx, unnanounced)
		if err != nil {
			// This could happen due to too big response (btcpay example with limited nginx), retry with other mode
			return getNodeInfoFullTemplate(ctx, l, math.MaxInt, channels, unnanounced)
		}
		for _, one := range graph.Channels {
			if one.Node1Pub != info.IdentityPubkey && one.Node2Pub != info.IdentityPubkey {
				continue
			}
			// No need to filter private channels (since we used unnanounced in DescribeGraph)
			private, ok := privateMapping[one.ChannelID]
			extendedNodeInfo.Channels = append(extendedNodeInfo.Channels, NodeChannelAPIExtended{NodeChannelAPI: one, Private: ok && private})
		}
	}

	return extendedNodeInfo, nil
}

// ErrorData struct
type ErrorData struct {
	Error          error
	IsStillRunning bool
}

// LightingAPICalls is the interface for lightning API
type LightingAPICalls interface {
	Cleanup()
	GetAPIType() APIType
	GetInfo(ctx context.Context) (*InfoAPI, error)
	GetChannels(ctx context.Context) (*ChannelsAPI, error)
	DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error)
	GetNodeInfoFull(ctx context.Context, channels, unannounced bool) (*NodeInfoAPIExtended, error)
	GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error)
	GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error)

	GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error)
	GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error)

	SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16) (<-chan []ForwardingEvent, <-chan ErrorData)

	GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error)
	GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error)
	GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error)
}

// GetDataCall - signature of function for retrieving data
type GetDataCall func() (*entities.Data, error)

// NewAPI - gets new lightning API
func NewAPI(apiType APIType, getData GetDataCall) LightingAPICalls {
	if getData == nil {
		sentry.CaptureMessage("getData was nil")
		return nil
	}

	data, err := getData()
	if err != nil {
		sentry.CaptureException(err)
		return nil
	}

	t := LndGrpc

	if data.ApiType != nil {
		foo, err := GetAPIType(data.ApiType)
		if err != nil {
			t = LndGrpc
		} else {
			t = *foo
		}

	} else {
		t = apiType
	}

	switch t {
	case LndGrpc:
		return NewLndGrpcLightningAPI(getData)
	case LndRest:
		return NewLndRestLightningAPI(getData)
	case ClnSocket:
		return NewClnSocketLightningAPI(getData)
	}

	sentry.CaptureMessage("Invalid api type")
	return nil
}
