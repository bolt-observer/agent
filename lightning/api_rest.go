package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
)

// LndRestLightningAPI struct
type LndRestLightningAPI struct {
	Request   *http.Request
	Transport *http.Transport
	HTTPAPI   *HTTPAPI
	Name      string
	API
}

// Compile time check for the interface
var _ LightingAPICalls = &LndRestLightningAPI{}

// NewLndRestLightningAPI constructs new lightning API
func NewLndRestLightningAPI(getData GetDataCall) LightingAPICalls {
	api := NewHTTPAPI()

	request, transport, err := api.GetHTTPRequest(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	api.SetTransport(transport)

	return &LndRestLightningAPI{
		Request:   request,
		Transport: transport,
		HTTPAPI:   api,
		Name:      "lndrest",
		API:       API{GetNodeInfoFullThreshUseDescribeGraph: 500},
	}
}

// GetInfo - GetInfo API
func (l *LndRestLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {

	resp, err := l.HTTPAPI.HTTPGetInfo(ctx, l.Request)

	if err != nil {
		return nil, err
	}

	ret := &InfoAPI{
		Alias:           resp.Alias,
		IdentityPubkey:  resp.IdentityPubkey,
		Chain:           resp.Chains[0].Chain,
		Network:         resp.Chains[0].Network,
		Version:         fmt.Sprintf("lnd-%s", resp.Version),
		IsSyncedToGraph: resp.SyncedToGraph,
		IsSyncedToChain: resp.SyncedToChain,
	}

	return ret, err
}

// Cleanup - clean up
func (l *LndRestLightningAPI) Cleanup() {
	// Nothing to do here
}

func stringToUint64(str string) uint64 {
	if str == "" {
		return 0
	}

	ret, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

func stringToInt64(str string) int64 {
	if str == "" {
		return 0
	}

	ret, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

// GetChannels - GetChannels API
func (l *LndRestLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetChannels(ctx, l.Request)

	if err != nil {
		return nil, err
	}

	chans := make([]ChannelAPI, 0)
	for _, channel := range resp.Channels {

		htlcs := make([]HtlcAPI, 0)
		for _, h := range channel.PendingHtlcs {
			htlcs = append(htlcs, HtlcAPI{
				Amount:              stringToUint64(h.Amount),
				Incoming:            h.Incoming,
				ForwardingChannel:   stringToUint64(h.ForwardingChannel),
				ForwardingHtlcIndex: stringToUint64(h.ForwardingHtlcIndex),
			})
		}

		chans = append(chans, ChannelAPI{
			Private:               channel.Private,
			Active:                channel.Active,
			RemotePubkey:          channel.RemotePubkey,
			ChanID:                stringToUint64(channel.ChanID),
			RemoteBalance:         stringToUint64(channel.RemoteBalance),
			LocalBalance:          stringToUint64(channel.LocalBalance),
			Capacity:              stringToUint64(channel.Capacity),
			PendingHtlcs:          htlcs,
			NumUpdates:            stringToUint64(channel.NumUpdates),
			CommitFee:             stringToUint64(channel.CommitFee),
			TotalSatoshisSent:     stringToUint64(channel.TotalSatoshisSent),
			TotalSatoshisReceived: stringToUint64(channel.TotalSatoshisReceived),
			Initiator:             channel.Initiator,
		})
	}

	ret := &ChannelsAPI{
		Channels: chans,
	}

	return ret, nil
}

func toPolicyWeb(policy *RoutingPolicyOverride) *RoutingPolicyAPI {
	if policy == nil {
		return nil
	}

	return &RoutingPolicyAPI{
		TimeLockDelta: policy.TimeLockDelta,
		MinHtlc:       stringToUint64(policy.MinHtlc),
		BaseFee:       stringToUint64(policy.FeeBaseMsat),
		FeeRate:       stringToUint64(policy.FeeRateMilliMsat),
		Disabled:      policy.Disabled,
		LastUpdate:    entities.JsonTime(time.Unix(int64(policy.LastUpdate), 0)),
		MaxHtlc:       stringToUint64(policy.MaxHtlcMsat),
	}
}

// DescribeGraph - DescribeGraph API
func (l *LndRestLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {

	resp, err := l.HTTPAPI.HTTPGetGraph(ctx, l.Request, unannounced)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeAPI, 0)

	for _, node := range resp.GraphNodeOverride {
		nodes = append(nodes, l.convertNode(node))
	}

	channels := make([]NodeChannelAPI, 0)

	for _, edge := range resp.GraphEdgesOverride {
		channels = append(channels, l.convertChan(edge))
	}

	ret := &DescribeGraphAPI{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}

func (l *LndRestLightningAPI) convertNode(node *GraphNodeOverride) DescribeGraphNodeAPI {
	addresses := make([]NodeAddressAPI, 0)
	for _, addr := range node.Addresses {
		addresses = append(addresses, NodeAddressAPI{Addr: addr.Addr, Network: addr.Network})
	}

	features := make(map[string]NodeFeatureAPI)
	for id, feat := range node.Features {
		features[fmt.Sprintf("%d", id)] = NodeFeatureAPI{Name: feat.Name, IsRequired: feat.IsRequired, IsKnown: feat.IsKnown}
	}

	return DescribeGraphNodeAPI{PubKey: node.PubKey, Alias: node.Alias, Color: node.Color, Features: features, Addresses: addresses,
		LastUpdate: entities.JsonTime(time.Unix(int64(node.LastUpdate), 0))}
}

func (l *LndRestLightningAPI) convertChan(edge *GraphEdgeOverride) NodeChannelAPI {
	return NodeChannelAPI{
		ChannelID:   stringToUint64(edge.ChannelID),
		ChanPoint:   edge.ChanPoint,
		Node1Pub:    edge.Node1Pub,
		Node2Pub:    edge.Node2Pub,
		Capacity:    stringToUint64(edge.Capacity),
		Node1Policy: toPolicyWeb(edge.Node1Policy),
		Node2Policy: toPolicyWeb(edge.Node2Policy),
		LastUpdate:  entities.JsonTime(time.Unix(int64(edge.LastUpdate), 0)),
	}
}

// GetNodeInfo - GetNodeInfo API
func (l *LndRestLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetNodeInfo(ctx, l.Request, pubKey, channels)
	if err != nil {
		return nil, err
	}

	ch := make([]NodeChannelAPI, 0)

	for _, edge := range resp.Channels {
		ch = append(ch, l.convertChan(edge))
	}

	ret := &NodeInfoAPI{Node: l.convertNode(resp.Node), Channels: ch, NumChannels: uint32(resp.NumChannels), TotalCapacity: stringToUint64(resp.TotalCapacity)}

	return ret, nil
}

// GetChanInfo - GetChanInfo API
func (l *LndRestLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetChanInfo(ctx, l.Request, chanID)
	if err != nil {
		return nil, err
	}
	ret := l.convertChan(resp)
	return &ret, nil
}

// SubscribeForwards - API call
func (l *LndRestLightningAPI) SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16, maxErrors uint16) (<-chan []ForwardingEvent, <-chan ErrorData) {
	panic("not implemented")
}

// GetInvoicesRaw - API call
func (l *LndRestLightningAPI) GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ListInvoiceRequestOverride{
		NumMaxInvoices: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset:    fmt.Sprintf("%d", pagination.Offset),
	}
	respPagination := &ResponseRawPagination{UseTimestamp: false}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	if pagination.From != nil || pagination.To != nil {
		return nil, respPagination, fmt.Errorf("from and to are not yet supported")
	}
	*/

	if pagination.Reversed {
		param.Reversed = true
	}

	if pendingOnly {
		param.PendingOnly = pendingOnly
	}

	resp, err := l.HTTPAPI.HTTPListInvoices(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	respPagination.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	ret := make([]RawMessage, 0, len(resp.Invoices))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, invoice := range resp.Invoices {
		t := time.Unix(stringToInt64(invoice.CreationDate), 0)
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(invoice)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetPaymentsRaw - API call
func (l *LndRestLightningAPI) GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ListPaymentsRequestOverride{
		MaxPayments: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset: fmt.Sprintf("%d", pagination.Offset),
	}
	respPagination := &ResponseRawPagination{UseTimestamp: false}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	if pagination.From != nil || pagination.To != nil {
		return nil, respPagination, fmt.Errorf("from and to are not yet supported")
	}
	*/

	if pagination.Reversed {
		param.Reversed = true
	}

	if includeIncomplete {
		param.IncludeIncomplete = includeIncomplete
	}

	resp, err := l.HTTPAPI.HTTPListPayments(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	respPagination.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	ret := make([]RawMessage, 0, len(resp.Payments))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, payment := range resp.Payments {
		t := time.Unix(stringToInt64(payment.CreationDate), 0)
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(payment)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetForwardsRaw - API call
func (l *LndRestLightningAPI) GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ForwardingHistoryRequestOverride{}

	param.NumMaxEvents = uint32(pagination.BatchSize)
	param.IndexOffset = uint32(pagination.Offset)

	if pagination.From != nil {
		param.StartTime = fmt.Sprintf("%d", pagination.From.Unix())
	}

	if pagination.To != nil {
		param.EndTime = fmt.Sprintf("%d", pagination.To.Unix())
	}

	respPagination := &ResponseRawPagination{UseTimestamp: false}

	resp, err := l.HTTPAPI.HTTPForwardEvents(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = uint64(resp.LastOffsetIndex)
	respPagination.FirstOffsetIndex = 0

	ret := make([]RawMessage, 0, len(resp.ForwardingEvents))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, forward := range resp.ForwardingEvents {
		t := time.Unix(0, stringToInt64(forward.TimestampNs))
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(forward)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetInvoices API
func (l *LndRestLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {

	param := &ListInvoiceRequestOverride{
		NumMaxInvoices: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset:    fmt.Sprintf("%d", pagination.Offset),
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		param.Reversed = true
	}

	if pendingOnly {
		param.PendingOnly = pendingOnly
	}

	resp, err := l.HTTPAPI.HTTPListInvoices(ctx, l.Request, param)
	if err != nil {
		return nil, err
	}

	ret := &InvoicesResponse{
		Invoices: make([]Invoice, 0, len(resp.Invoices)),
	}

	ret.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	ret.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	for _, invoice := range resp.Invoices {
		ret.Invoices = append(ret.Invoices, Invoice{
			Memo:            invoice.Memo,
			ValueMsat:       stringToInt64(invoice.ValueMsat),
			PaidMsat:        stringToInt64(invoice.AmtPaidMsat),
			CreationDate:    time.Unix(stringToInt64(invoice.CreationDate), 0),
			SettleDate:      time.Unix(stringToInt64(invoice.SettleDate), 0),
			PaymentRequest:  invoice.PaymentRequest,
			DescriptionHash: string(invoice.DescriptionHash),
			Expiry:          stringToInt64(invoice.Expiry),
			FallbackAddr:    invoice.FallbackAddr,
			CltvExpiry:      stringToUint64(invoice.CltvExpiry),
			Private:         invoice.Private,
			IsKeySend:       invoice.IsKeysend,
			IsAmp:           invoice.IsAmp,
			State:           StringToInvoiceHTLCState(invoice.State),
			AddIndex:        stringToUint64(invoice.AddIndex),
			SettleIndex:     stringToUint64(invoice.SettleIndex),
		})
	}

	return ret, nil
}

// GetPayments API
func (l *LndRestLightningAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	param := &ListPaymentsRequestOverride{
		MaxPayments: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset: fmt.Sprintf("%d", pagination.Offset),
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		param.Reversed = true
	}

	if includeIncomplete {
		param.IncludeIncomplete = includeIncomplete
	}

	resp, err := l.HTTPAPI.HTTPListPayments(ctx, l.Request, param)
	if err != nil {
		return nil, err
	}

	ret := &PaymentsResponse{
		Payments: make([]Payment, 0, len(resp.Payments)),
	}

	return ret, nil
}

// SubscribeFailedForwards is used to subscribe to failed forwards
func (l *LndRestLightningAPI) SubscribeFailedForwards(ctx context.Context, outchan chan RawMessage) error {

	go func() {
		resp, err := l.HTTPAPI.HTTPSubscribeHtlcEvents(ctx, l.Request)
		if err != nil {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Do nothing
			}

			event := <-resp

			if !strings.Contains(strings.ToLower(event.EventType), "forward") {
				continue
			}

			if event.Event.FailureString == "" {
				continue
			}

			m := RawMessage{
				Implementation: l.Name,
				Timestamp:      time.Unix(0, stringToInt64(event.TimestampNs)),
			}

			m.Message, err = json.Marshal(event)
			if err != nil {
				continue
			}

			outchan <- m
		}
	}()

	return nil
}

// GetAPIType API
func (l *LndRestLightningAPI) GetAPIType() APIType {
	return LndRest
}
