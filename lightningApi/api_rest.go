package lightningapi

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
	LightningApi
}

// Compile time check for the interface
var _ LightingApiCalls = &LndRestLightningAPI{}

// NewLndRestLightningAPI constructs new lightning API
func NewLndRestLightningAPI(getData GetDataCall) LightingApiCalls {
	api := NewHTTPAPI()

	request, transport, err := api.GetHttpRequest(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	api.SetTransport(transport)

	return &LndRestLightningAPI{
		Request:      request,
		Transport:    transport,
		HTTPAPI:      api,
		LightningApi: LightningApi{GetNodeInfoFullThreshUseDescribeGraph: 500},
	}
}

// GetInfo - GetInfo API
func (l *LndRestLightningAPI) GetInfo(ctx context.Context) (*InfoApi, error) {

	resp, err := l.HTTPAPI.HttpGetInfo(ctx, l.Request, l.Transport)

	if err != nil {
		return nil, err
	}

	ret := &InfoApi{
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
func (l *LndRestLightningAPI) GetChannels(ctx context.Context) (*ChannelsApi, error) {
	resp, err := l.HTTPAPI.HttpGetChannels(ctx, l.Request, l.Transport)

	if err != nil {
		return nil, err
	}

	chans := make([]ChannelApi, 0)
	for _, channel := range resp.Channels {

		htlcs := make([]HtlcApi, 0)
		for _, h := range channel.PendingHtlcs {
			htlcs = append(htlcs, HtlcApi{
				Amount:              stringToUint64(h.Amount),
				Incoming:            h.Incoming,
				ForwardingChannel:   stringToUint64(h.ForwardingChannel),
				ForwardingHtlcIndex: stringToUint64(h.ForwardingHtlcIndex),
			})
		}

		chans = append(chans, ChannelApi{
			Private:               channel.Private,
			Active:                channel.Active,
			RemotePubkey:          channel.RemotePubkey,
			ChanId:                stringToUint64(channel.ChanID),
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

	ret := &ChannelsApi{
		Channels: chans,
	}

	return ret, nil
}

func toPolicyWeb(policy *RoutingPolicyOverride) *RoutingPolicyApi {
	if policy == nil {
		return nil
	}

	return &RoutingPolicyApi{
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
func (l *LndRestLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error) {

	resp, err := l.HTTPAPI.HttpGetGraph(ctx, l.Request, l.Transport, unannounced)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeApi, 0)

	for _, node := range resp.GraphNodeOverride {
		nodes = append(nodes, l.convertNode(node))
	}

	channels := make([]NodeChannelApi, 0)

	for _, edge := range resp.GraphEdgesOverride {
		channels = append(channels, l.convertChan(edge))
	}

	ret := &DescribeGraphApi{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}

func (l *LndRestLightningAPI) convertNode(node *GraphNodeOverride) DescribeGraphNodeApi {
	addresses := make([]NodeAddressApi, 0)
	for _, addr := range node.Addresses {
		addresses = append(addresses, NodeAddressApi{Addr: addr.Addr, Network: addr.Network})
	}

	features := make(map[string]NodeFeatureApi)
	for id, feat := range node.Features {
		features[fmt.Sprintf("%d", id)] = NodeFeatureApi{Name: feat.Name, IsRequired: feat.IsRequired, IsKnown: feat.IsKnown}
	}

	return DescribeGraphNodeApi{PubKey: node.PubKey, Alias: node.Alias, Color: node.Color, Features: features, Addresses: addresses,
		LastUpdate: entities.JsonTime(time.Unix(int64(node.LastUpdate), 0))}
}

func (l *LndRestLightningAPI) convertChan(edge *GraphEdgeOverride) NodeChannelApi {
	return NodeChannelApi{
		ChannelId:   stringToUint64(edge.ChannelID),
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
func (l *LndRestLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoApi, error) {
	resp, err := l.HTTPAPI.HttpGetNodeInfo(ctx, l.Request, l.Transport, pubKey, channels)
	if err != nil {
		return nil, err
	}

	ch := make([]NodeChannelApi, 0)

	for _, edge := range resp.Channels {
		ch = append(ch, l.convertChan(edge))
	}

	ret := &NodeInfoApi{Node: l.convertNode(resp.Node), Channels: ch, NumChannels: uint32(resp.NumChannels), TotalCapacity: stringToUint64(resp.TotalCapacity)}

	return ret, nil
}

// GetChanInfo - GetChanInfo API
func (l *LndRestLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelApi, error) {
	resp, err := l.HTTPAPI.HttpGetChanInfo(ctx, l.Request, l.Transport, chanID)
	if err != nil {
		return nil, err
	}
	ret := l.convertChan(resp)
	return &ret, nil
}
