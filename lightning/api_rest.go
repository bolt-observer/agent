package lightning

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
)

// LndRestLightningAPI struct
type LndRestLightningAPI struct {
	Request   *http.Request
	Transport *http.Transport
	HTTPAPI   *HTTPAPI
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
		Request:      request,
		Transport:    transport,
		HTTPAPI:      api,
		API: API{GetNodeInfoFullThreshUseDescribeGraph: 500},
	}
}

// GetInfo - GetInfo API
func (l *LndRestLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {

	resp, err := l.HTTPAPI.HTTPGetInfo(ctx, l.Request, l.Transport)

	if err != nil {
		return nil, err
	}

	ret := &InfoAPI{
		Alias:          resp.Alias,
		IdentityPubkey: resp.IdentityPubkey,
		Chain:          resp.Chains[0].Chain,
		Network:        resp.Chains[0].Network,
	}

	return ret, err
}

// Cleanup - clean up
func (l *LndRestLightningAPI) Cleanup() {
	// Nothing to do here
}

func stringToUint64(str string) uint64 {
	ret, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

// GetChannels - GetChannels API
func (l *LndRestLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetChannels(ctx, l.Request, l.Transport)

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

	resp, err := l.HTTPAPI.HTTPGetGraph(ctx, l.Request, l.Transport, unannounced)
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
	resp, err := l.HTTPAPI.HTTPGetNodeInfo(ctx, l.Request, l.Transport, pubKey, channels)
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
	resp, err := l.HTTPAPI.HTTPGetChanInfo(ctx, l.Request, l.Transport, chanID)
	if err != nil {
		return nil, err
	}
	ret := l.convertChan(resp)
	return &ret, nil
}

// GetForwardingHistory API
func (l *LndRestLightningAPI) GetForwardingHistory(ctx context.Context, pagination Pagination) (*ForwardingHistoryResponse, error) {

	param := &ForwardingHistoryRequestOverride{}

	param.NumMaxEvents = uint32(pagination.Num)
	param.IndexOffset = uint32(pagination.Offset)

	if pagination.From != nil {
		param.StartTime = fmt.Sprintf("%d", uint64(pagination.From.Unix()))
	}

	if pagination.To != nil {
		param.EndTime = fmt.Sprintf("%d", uint64(pagination.To.Unix()))
	}

	if pagination.Reversed {
		return nil, fmt.Errorf("reverse pagination not supported")
	}

	resp, err := l.HTTPAPI.HTTPForwardEvents(ctx, l.Request, l.Transport, param)
	if err != nil {
		return nil, err
	}

	ret := &ForwardingHistoryResponse{
		LastOffsetIndex:  uint64(resp.LastOffsetIndex),
		ForwardingEvents: make([]ForwardingEvent, 0, len(resp.ForwardingEvents)),
	}

	for _, event := range resp.ForwardingEvents {
		ret.ForwardingEvents = append(ret.ForwardingEvents, ForwardingEvent{
			Timestamp:     time.Unix(0, int64(stringToUint64(event.TimestampNs))),
			ChanIDIn:      stringToUint64(event.ChanIDIn),
			ChanIDOut:     stringToUint64(event.ChanIDOut),
			AmountInMsat:  stringToUint64(event.AmtInMsat),
			AmountOutMsat: stringToUint64(event.AmtOutMsat),
			FeeMsat:       stringToUint64(event.FeeMsat),
		})
	}

	return ret, nil
}

// GetInvoices API
func (l *LndRestLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	panic("not implemented")
}
