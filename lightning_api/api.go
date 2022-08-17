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
	Channels []DescribeGraphChannelApi
}

type DescribeGraphNodeApi struct {
	PubKey string
	Alias  string
}

type DescribeGraphChannelApi struct {
	ChannelId   uint64
	ChanPoint   string
	Node1Pub    string
	Node2Pub    string
	Capacity    uint64
	Node1Policy *RoutingPolicyApi
	Node2Policy *RoutingPolicyApi
}

type RoutingPolicyApi struct {
	TimeLockDelta uint32
	MinHtlc       uint64
	BaseFee       uint64
	FeeRate       uint64
	Disabled      bool
	LastUpdate    time.Time
	MaxHtlc       uint64
}

type LightningApi struct {
}

type LightingApiCalls interface {
	Cleanup()
	GetInfo(ctx context.Context) (*InfoApi, error)
	GetChannels(ctx context.Context) (*ChannelsApi, error)
	DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error)
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
