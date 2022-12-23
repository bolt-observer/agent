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

// LndGrpcLightningAPI struct
type LndGrpcLightningAPI struct {
	Client      lnrpc.LightningClient
	CleanupFunc func()
	LightningAPI
}

// Compile time check for the interface
var _ LightingAPICalls = &LndGrpcLightningAPI{}

// NewLndGrpcLightningAPI - creates new lightning API
func NewLndGrpcLightningAPI(getData GetDataCall) LightingAPICalls {
	client, cleanup, err := GetClient(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	return &LndGrpcLightningAPI{
		Client:       client,
		CleanupFunc:  cleanup,
		LightningAPI: LightningAPI{GetNodeInfoFullThreshUseDescribeGraph: 500},
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

// GetInfo API
func (l *LndGrpcLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	resp, err := l.Client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
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

// Cleanup API
func (l *LndGrpcLightningAPI) Cleanup() {
	l.CleanupFunc()
}

// GetChannels API
func (l *LndGrpcLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	resp, err := l.Client.ListChannels(ctx, &lnrpc.ListChannelsRequest{})

	if err != nil {
		return nil, err
	}

	chans := make([]ChannelAPI, 0)
	for _, channel := range resp.Channels {

		htlcs := make([]HtlcAPI, 0)
		for _, h := range channel.PendingHtlcs {
			htlcs = append(htlcs, HtlcAPI{
				Amount:              uint64(h.Amount),
				Incoming:            h.Incoming,
				ForwardingChannel:   h.ForwardingChannel,
				ForwardingHtlcIndex: h.ForwardingHtlcIndex,
			})
		}

		chans = append(chans, ChannelAPI{
			Private:               channel.Private,
			Active:                channel.Active,
			RemotePubkey:          channel.RemotePubkey,
			ChanID:                channel.ChanId,
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

	ret := &ChannelsAPI{
		Channels: chans,
	}

	return ret, nil
}

func toPolicy(policy *lnrpc.RoutingPolicy) *RoutingPolicyAPI {
	if policy == nil {
		return nil
	}

	return &RoutingPolicyAPI{
		TimeLockDelta: policy.TimeLockDelta,
		MinHtlc:       uint64(policy.MinHtlc),
		BaseFee:       uint64(policy.FeeBaseMsat),
		FeeRate:       uint64(policy.FeeRateMilliMsat),
		Disabled:      policy.Disabled,
		LastUpdate:    entities.JsonTime(time.Unix(int64(policy.LastUpdate), 0)),
		MaxHtlc:       policy.MaxHtlcMsat,
	}
}

// DescribeGraph API
func (l *LndGrpcLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	resp, err := l.Client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{IncludeUnannounced: unannounced})

	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeAPI, 0)

	for _, node := range resp.Nodes {
		nodes = append(nodes, l.convertNode(node))
	}

	channels := make([]NodeChannelAPI, 0)

	for _, edge := range resp.Edges {
		channels = append(channels, l.convertChan(edge))
	}

	ret := &DescribeGraphAPI{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}

func (l *LndGrpcLightningAPI) convertNode(node *lnrpc.LightningNode) DescribeGraphNodeAPI {
	addresses := make([]NodeAddressAPI, 0)
	for _, addr := range node.Addresses {
		addresses = append(addresses, NodeAddressAPI{Addr: addr.Addr, Network: addr.Network})
	}

	features := make(map[string]NodeFeatureAPI)
	for id, feat := range node.Features {
		features[fmt.Sprintf("%d", id)] = NodeFeatureAPI{Name: feat.Name, IsRequired: feat.IsRequired, IsKnown: feat.IsKnown}
	}

	return DescribeGraphNodeAPI{PubKey: node.PubKey, Alias: node.Alias, Color: node.Color, Addresses: addresses, Features: features,
		LastUpdate: entities.JsonTime(time.Unix(int64(node.LastUpdate), 0))}
}

func (l *LndGrpcLightningAPI) convertChan(edge *lnrpc.ChannelEdge) NodeChannelAPI {
	return NodeChannelAPI{
		ChannelID:   edge.ChannelId,
		ChanPoint:   edge.ChanPoint,
		Node1Pub:    edge.Node1Pub,
		Node2Pub:    edge.Node2Pub,
		Capacity:    uint64(edge.Capacity),
		Node1Policy: toPolicy(edge.Node1Policy),
		Node2Policy: toPolicy(edge.Node2Policy),
		LastUpdate:  entities.JsonTime(time.Unix(int64(edge.LastUpdate), 0)),
	}
}

// GetNodeInfo API
func (l *LndGrpcLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	resp, err := l.Client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{PubKey: pubKey, IncludeChannels: channels})

	if err != nil {
		return nil, err
	}

	ch := make([]NodeChannelAPI, 0)

	for _, edge := range resp.Channels {
		ch = append(ch, l.convertChan(edge))
	}

	ret := &NodeInfoAPI{Node: l.convertNode(resp.Node), Channels: ch, NumChannels: resp.NumChannels, TotalCapacity: uint64(resp.TotalCapacity)}
	return ret, nil
}

// GetChanInfo API
func (l *LndGrpcLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {
	resp, err := l.Client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{ChanId: chanID})

	if err != nil {
		return nil, err
	}

	ret := l.convertChan(resp)
	return &ret, nil
}
