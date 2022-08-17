package lightning_api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
)

type LndRestLightningApi struct {
	Request   *http.Request
	Transport *http.Transport
	HttpApi   *HttpApi
	LightningApi
}

func NewLndRestLightningApi(getData GetDataCall) LightingApiCalls {
	api := NewHttpApi()

	request, transport, err := api.GetHttpRequest(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	api.SetTransport(transport)

	return &LndRestLightningApi{
		Request:   request,
		Transport: transport,
		HttpApi:   api,
	}
}

func (l *LndRestLightningApi) GetInfo(ctx context.Context) (*InfoApi, error) {

	resp, err := l.HttpApi.HttpGetInfo(ctx, l.Request, l.Transport)

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

func (l *LndRestLightningApi) Cleanup() {
	// Nothing to do here
}

func stringToUint64(str string) uint64 {
	ret, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

func (l *LndRestLightningApi) GetChannels(ctx context.Context) (*ChannelsApi, error) {
	resp, err := l.HttpApi.HttpGetChannels(ctx, l.Request, l.Transport)

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
			ChanId:                stringToUint64(channel.ChanId),
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
		LastUpdate:    time.Unix(int64(policy.LastUpdate), 0),
		MaxHtlc:       stringToUint64(policy.MaxHtlcMsat),
	}
}

func (l *LndRestLightningApi) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error) {

	resp, err := l.HttpApi.HttpGetGraph(ctx, l.Request, l.Transport, unannounced)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeApi, 0)

	for _, node := range resp.GraphNodeOverride {
		nodes = append(nodes, DescribeGraphNodeApi{PubKey: node.PubKey, Alias: node.Alias})
	}

	channels := make([]DescribeGraphChannelApi, 0)

	for _, edge := range resp.GraphEdgesOverride {
		channels = append(channels, DescribeGraphChannelApi{
			ChannelId:   stringToUint64(edge.ChannelId),
			ChanPoint:   edge.ChanPoint,
			Node1Pub:    edge.Node1Pub,
			Node2Pub:    edge.Node2Pub,
			Capacity:    stringToUint64(edge.Capacity),
			Node1Policy: toPolicyWeb(edge.Node1Policy),
			Node2Policy: toPolicyWeb(edge.Node2Policy),
		})
	}

	ret := &DescribeGraphApi{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}
