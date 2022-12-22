package lightningapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
	"github.com/lightningnetwork/lnd/lnrpc"
)

type LndGrpcLightningApi struct {
	Client      lnrpc.LightningClient
	CleanupFunc func()
	LightningApi
}

// Compile time check for the interface
var _ LightingApiCalls = &LndGrpcLightningApi{}

func NewLndGrpcLightningApi(getData GetDataCall) LightingApiCalls {
	client, cleanup, err := GetClient(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	return &LndGrpcLightningApi{
		Client:       client,
		CleanupFunc:  cleanup,
		LightningApi: LightningApi{GetNodeInfoFullThreshUseDescribeGraph: 500},
	}
}

// Not used
func debugOutput(resp *lnrpc.ChannelEdge) {
	bodyData, _ := json.Marshal(resp)
	f, _ := os.OpenFile("dummy.json", os.O_WRONLY|os.O_CREATE, 0644)
	f.Truncate(0)
	defer f.Close()
	json.Unmarshal(bodyData, &resp)
	fmt.Fprintf(f, "%s\n", string(bodyData))
}

func (l *LndGrpcLightningApi) GetInfo(ctx context.Context) (*InfoApi, error) {
	resp, err := l.Client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
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

func (l *LndGrpcLightningApi) Cleanup() {
	l.CleanupFunc()
}

func (l *LndGrpcLightningApi) GetChannels(ctx context.Context) (*ChannelsApi, error) {
	resp, err := l.Client.ListChannels(ctx, &lnrpc.ListChannelsRequest{})

	if err != nil {
		return nil, err
	}

	chans := make([]ChannelApi, 0)
	for _, channel := range resp.Channels {

		htlcs := make([]HtlcApi, 0)
		for _, h := range channel.PendingHtlcs {
			htlcs = append(htlcs, HtlcApi{
				Amount:              uint64(h.Amount),
				Incoming:            h.Incoming,
				ForwardingChannel:   h.ForwardingChannel,
				ForwardingHtlcIndex: h.ForwardingHtlcIndex,
			})
		}

		chans = append(chans, ChannelApi{
			Private:               channel.Private,
			Active:                channel.Active,
			RemotePubkey:          channel.RemotePubkey,
			ChanId:                channel.ChanId,
			RemoteBalance:         uint64(channel.RemoteBalance),
			LocalBalance:          uint64(channel.LocalBalance),
			Capacity:              uint64(channel.Capacity),
			PendingHtlcs:          htlcs,
			NumUpdates:            channel.NumUpdates,
			CommitFee:             uint64(channel.CommitFee),
			TotalSatoshisSent:     uint64(channel.TotalSatoshisSent),
			TotalSatoshisReceived: uint64(channel.TotalSatoshisReceived),
			Initiator:             channel.Initiator,
		})
	}

	ret := &ChannelsApi{
		Channels: chans,
	}

	return ret, nil
}

func toPolicy(policy *lnrpc.RoutingPolicy) *RoutingPolicyApi {
	if policy == nil {
		return nil
	}

	return &RoutingPolicyApi{
		TimeLockDelta: policy.TimeLockDelta,
		MinHtlc:       uint64(policy.MinHtlc),
		BaseFee:       uint64(policy.FeeBaseMsat),
		FeeRate:       uint64(policy.FeeRateMilliMsat),
		Disabled:      policy.Disabled,
		LastUpdate:    entities.JsonTime(time.Unix(int64(policy.LastUpdate), 0)),
		MaxHtlc:       policy.MaxHtlcMsat,
	}
}

func (l *LndGrpcLightningApi) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error) {
	resp, err := l.Client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{IncludeUnannounced: unannounced})

	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeApi, 0)

	for _, node := range resp.Nodes {
		nodes = append(nodes, l.convertNode(node))
	}

	channels := make([]NodeChannelApi, 0)

	for _, edge := range resp.Edges {
		channels = append(channels, l.convertChan(edge))
	}

	ret := &DescribeGraphApi{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}

func (l *LndGrpcLightningApi) convertNode(node *lnrpc.LightningNode) DescribeGraphNodeApi {
	addresses := make([]NodeAddressApi, 0)
	for _, addr := range node.Addresses {
		addresses = append(addresses, NodeAddressApi{Addr: addr.Addr, Network: addr.Network})
	}

	features := make(map[string]NodeFeatureApi)
	for id, feat := range node.Features {
		features[fmt.Sprintf("%d", id)] = NodeFeatureApi{Name: feat.Name, IsRequired: feat.IsRequired, IsKnown: feat.IsKnown}
	}

	return DescribeGraphNodeApi{PubKey: node.PubKey, Alias: node.Alias, Color: node.Color, Addresses: addresses, Features: features,
		LastUpdate: entities.JsonTime(time.Unix(int64(node.LastUpdate), 0))}
}

func (l *LndGrpcLightningApi) convertChan(edge *lnrpc.ChannelEdge) NodeChannelApi {
	return NodeChannelApi{
		ChannelId:   edge.ChannelId,
		ChanPoint:   edge.ChanPoint,
		Node1Pub:    edge.Node1Pub,
		Node2Pub:    edge.Node2Pub,
		Capacity:    uint64(edge.Capacity),
		Node1Policy: toPolicy(edge.Node1Policy),
		Node2Policy: toPolicy(edge.Node2Policy),
		LastUpdate:  entities.JsonTime(time.Unix(int64(edge.LastUpdate), 0)),
	}
}

func (l *LndGrpcLightningApi) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoApi, error) {
	resp, err := l.Client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{PubKey: pubKey, IncludeChannels: channels})

	if err != nil {
		return nil, err
	}

	ch := make([]NodeChannelApi, 0)

	for _, edge := range resp.Channels {
		ch = append(ch, l.convertChan(edge))
	}

	ret := &NodeInfoApi{Node: l.convertNode(resp.Node), Channels: ch, NumChannels: resp.NumChannels, TotalCapacity: uint64(resp.TotalCapacity)}
	return ret, nil
}

func (l *LndGrpcLightningApi) GetChanInfo(ctx context.Context, chanId uint64) (*NodeChannelApi, error) {
	resp, err := l.Client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{ChanId: chanId})

	if err != nil {
		return nil, err
	}

	ret := l.convertChan(resp)
	return &ret, nil
}
