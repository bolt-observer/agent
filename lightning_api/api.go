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
	TimeLockDelta uint32    `json:"time_lock_delta,omitempty"`
	MinHtlc       uint64    `json:"min_htlc,omitempty"`
	BaseFee       uint64    `json:"fee_base_msat,omitempty"`
	FeeRate       uint64    `json:"fee_rate_milli_msat,omitempty"`
	Disabled      bool      `json:"disabled,omitempty"`
	LastUpdate    time.Time `json:"-"`
	MaxHtlc       uint64    `json:"max_htlc_msat,omitempty"`
}

type NodeInfoApi struct {
	Node          DescribeGraphNodeApi `json:"node,omitempty"`
	Channels      []NodeChannelApi     `json:"channels"`
	NumChannels   uint32               `json:"num_channels,omitempty"`
	TotalCapacity uint64               `json:"total_capacity,omitempty"`
}

type NodeChannelApiExtended struct {
	Private bool `json:"private,omitempty"`
	NodeChannelApi
}

type NodeInfoApiExtended struct {
	NodeInfoApi
	Channels []NodeChannelApiExtended `json:"channels"`
}

type LightningApi struct {
	GetNodeInfoFullThreshUseDescribeGraph int // If node has more than that number of channels use DescribeGraph else do GetChanInfo for each one
}

func (l *LndGrpcLightningApi) GetNodeInfoFull(ctx context.Context, channels, unnanounced bool) (*NodeInfoApiExtended, error) {
	return getNodeInfoFull(l, l.GetNodeInfoFullThreshUseDescribeGraph, ctx, channels, unnanounced)
}

func (l *LndRestLightningApi) GetNodeInfoFull(ctx context.Context, channels, unnanounced bool) (*NodeInfoApiExtended, error) {
	return getNodeInfoFull(l, l.GetNodeInfoFullThreshUseDescribeGraph, ctx, channels, unnanounced)
}

// GetNodeInfoFull returns info for local node possibly including unnanounced channels (as soon as that can be obtained via GetNodeInfo this method is useless)
func getNodeInfoFull(l LightingApiCalls, threshUseDescribeGraph int, ctx context.Context, channels, unnanounced bool) (*NodeInfoApiExtended, error) {
	info, err := l.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := l.GetNodeInfo(ctx, info.IdentityPubkey, channels)
	if err != nil {
		return nil, err
	}

	extendedNodeInfo := &NodeInfoApiExtended{NodeInfoApi: *nodeInfo}

	if !unnanounced {
		// We have full info already (fast bailout)
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

		privateMapping[ch.ChanId] = ch.Private
		totalCapacity += ch.Capacity
		numChans += 1
	}

	nodeInfo.NumChannels = uint32(numChans)
	nodeInfo.TotalCapacity = totalCapacity

	if !channels {
		return extendedNodeInfo, nil
	}

	extendedNodeInfo.Channels = make([]NodeChannelApiExtended, 0)

	if len(chans.Channels) <= threshUseDescribeGraph {
		for _, ch := range chans.Channels {
			if ch.Private && !unnanounced {
				continue
			}
			c, err := l.GetChanInfo(ctx, ch.ChanId)
			if err != nil {
				return nil, err
			}
			private, ok := privateMapping[ch.ChanId]
			extendedNodeInfo.Channels = append(extendedNodeInfo.Channels, NodeChannelApiExtended{NodeChannelApi: *c, Private: ok && private})
		}
	} else {
		graph, err := l.DescribeGraph(ctx, unnanounced)
		if err != nil {
			return nil, err
		}
		for _, one := range graph.Channels {
			if one.Node1Pub != info.IdentityPubkey && one.Node2Pub != info.IdentityPubkey {
				continue
			}
			// No need to filter private channels (since we used unnanounced in DescribeGraph)
			private, ok := privateMapping[one.ChannelId]
			extendedNodeInfo.Channels = append(extendedNodeInfo.Channels, NodeChannelApiExtended{NodeChannelApi: one, Private: ok && private})
		}
	}

	return extendedNodeInfo, nil
}

type LightingApiCalls interface {
	Cleanup()
	GetInfo(ctx context.Context) (*InfoApi, error)
	GetChannels(ctx context.Context) (*ChannelsApi, error)
	DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error)
	GetNodeInfoFull(ctx context.Context, channels, unannounced bool) (*NodeInfoApiExtended, error)
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
