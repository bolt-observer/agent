package lightning_api

import (
	"context"
	"fmt"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
)

type ApiType int

const (
	LND_GRPC ApiType = iota
	LND_REST
)

func GetApiType(t *int) (*ApiType, error) {
	if t == nil {
		return nil, fmt.Errorf("no api type specified")
	}
	if *t != int(LND_GRPC) && *t != int(LND_REST) {
		return nil, fmt.Errorf("invalid api type specified")
	}

	ret := ApiType(*t)
	return &ret, nil
}

type InfoApi struct {
	IdentityPubkey string
	Alias          string
	Chain          string
	Network        string
}

type ChannelsApi struct {
	Channels []ChannelApi
}

type ChannelApi struct {
	Private               bool
	Active                bool
	RemotePubkey          string
	Initiator             bool
	CommitFee             uint64
	ChanId                uint64
	RemoteBalance         uint64
	LocalBalance          uint64
	Capacity              uint64
	PendingHtlcs          []HtlcApi
	TotalSatoshisSent     uint64
	TotalSatoshisReceived uint64
	NumUpdates            uint64
}

type HtlcApi struct {
	Amount              uint64
	Incoming            bool
	ForwardingChannel   uint64
	ForwardingHtlcIndex uint64
}

type DescribeGraphApi struct {
	Nodes    []DescribeGraphNodeApi
	Channels []NodeChannelApi
}

type DescribeGraphNodeApi struct {
	PubKey    string                    `json:"pub_key,omitempty"`
	Alias     string                    `json:"alias,omitempty"`
	Color     string                    `json:"color,omitempty"`
	Addresses []NodeAddressApi          `json:"addresses,omitempty"`
	Features  map[string]NodeFeatureApi `json:"features,omitempty"`
}

type NodeAddressApi struct {
	Network string `json:"network,omitempty"`
	Addr    string `json:"addr,omitempty"`
}

type NodeFeatureApi struct {
	Name       string `json:"name,omitempty"`
	IsRequired bool   `json:"is_required,omitempty"`
	IsKnown    bool   `json:"is_known,omitempty"`
}

type NodeChannelApi struct {
	ChannelId   uint64            `json:"channel_id,omitempty"`
	ChanPoint   string            `json:"chan_point,omitempty"`
	Node1Pub    string            `json:"node1_pub,omitempty"`
	Node2Pub    string            `json:"node2_pub,omitempty"`
	Capacity    uint64            `json:"capacity,omitempty"`
	Node1Policy *RoutingPolicyApi `json:"node1_policy,omitempty"`
	Node2Policy *RoutingPolicyApi `json:"node2_policy,omitempty"`
}

type RoutingPolicyApi struct {
	TimeLockDelta uint32 `json:"time_lock_delta,omitempty"`
	MinHtlc       uint64 `json:"min_htlc,omitempty"`
	BaseFee       uint64 `json:"fee_base_msat,omitempty"`
	FeeRate       uint64 `json:"fee_rate_milli_msat,omitempty"`
	Disabled      bool   `json:"disabled,omitempty"`
	LastUpdate    time.Time
	MaxHtlc       uint64 `json:"max_htlc_msat,omitempty"`
}

type NodeInfoApi struct {
	Node          DescribeGraphNodeApi `json:"node,omitempty"`
	Channels      []NodeChannelApi     `json:"channels"`
	NumChannels   uint32               `json:"num_channels,omitempty"`
	TotalCapacity uint64               `json:"total_capacity,omitempty"`
}

type LightningApi struct {
}

func (l *LndGrpcLightningApi) GetNodeInfoFull(ctx context.Context) (*NodeInfoApi, error) {
	return GetNodeInfoFull(l, ctx)
}

func (l *LndRestLightningApi) GetNodeInfoFull(ctx context.Context) (*NodeInfoApi, error) {
	return GetNodeInfoFull(l, ctx)
}

func GetNodeInfoFull(l LightingApiCalls, ctx context.Context) (*NodeInfoApi, error) {
	info, err := l.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := l.GetNodeInfo(ctx, info.IdentityPubkey, false)
	if err != nil {
		return nil, err
	}

	nodeInfo.Channels = make([]NodeChannelApi, 0)
	chans, err := l.GetChannels(ctx)
	if err != nil {
		return nil, err
	}

	numChans := 0
	totalCapacity := uint64(0)

	for _, ch := range chans.Channels {
		c, err := l.GetChanInfo(ctx, ch.ChanId)
		if err != nil {
			return nil, err
		}
		totalCapacity += c.Capacity
		numChans += 1

		nodeInfo.Channels = append(nodeInfo.Channels, *c)
	}

	nodeInfo.NumChannels = uint32(numChans)
	nodeInfo.TotalCapacity = totalCapacity

	return nodeInfo, nil
}

type LightingApiCalls interface {
	Cleanup()
	GetInfo(ctx context.Context) (*InfoApi, error)
	GetChannels(ctx context.Context) (*ChannelsApi, error)
	DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error)
	GetNodeInfoFull(ctx context.Context) (*NodeInfoApi, error)
	GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoApi, error)
	GetChanInfo(ctx context.Context, chanId uint64) (*NodeChannelApi, error)
}

type GetDataCall func() (*entities.Data, error)

// Get new API
func NewApi(apiType ApiType, getData GetDataCall) LightingApiCalls {
	if getData == nil {
		return nil
	}

	data, err := getData()
	if err != nil {
		return nil
	}

	t := LND_GRPC

	if data.ApiType != nil {
		foo, err := GetApiType(data.ApiType)
		if err != nil {
			t = LND_GRPC
		} else {
			t = *foo
		}

	} else {
		t = apiType
	}

	switch t {
	case LND_GRPC:
		return NewLndGrpcLightningApi(getData)
	case LND_REST:
		return NewLndRestLightningApi(getData)
	}

	return nil
}
