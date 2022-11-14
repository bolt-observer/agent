package lightning_api

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

var (
	ErrNoNode          = fmt.Errorf("node not found")
	ErrNoChan          = fmt.Errorf("channel not found")
	ErrInvalidResponse = fmt.Errorf("invalid response")
)

type ClnInfo struct {
	PubKey    string            `json:"id"`
	Alias     string            `json:"alias"`
	Color     string            `json:"color"`
	Network   string            `json:"network"`
	Addresses []ClnListNodeAddr `json:"address,omitempty"`
	Features  ClnFeatures       `json:"our_features"`
}

type ClnFeatures struct {
	Init    string `json:"init"`
	Node    string `json:"node"`
	Channel string `json:"channe"`
	Invoice string `json:"invoice"`
}

type ClnSetChan struct {
	PeerId         string `json:"peer_id"`
	LongChanId     string `json:"channel_id"`
	FeeBase        string `json:"fee_base_msat"`
	FeePpm         string `json:"fee_proportional_milli"`
	MiHtlc         string `json:"minimum_htlc_out_msat"`
	MaxHtlc        string `json:"maximum_htlc_out_msat"`
	ShortChannelId string `json:"short_channel_id,omitempty"`
}

type ClnSetChanResp struct {
	Settings []ClnSetChan `json:"channels,omitempty"`
}

type ClnListChan struct {
	Source         string            `json:"source"`
	Destination    string            `json:"destination"`
	Public         bool              `json:"public"`
	Capacity       uint64            `json:"satoshis"`
	Active         bool              `json:"active"`
	LastUpdate     entities.JsonTime `json:"last_update"`
	FeeBase        uint64            `json:"base_fee_millisatoshi"`
	FeePpm         uint64            `json:"fee_per_millionth"`
	MinHtlc        string            `json:"htlc_minimum_msat"`
	MaxHtlc        string            `json:"htlc_maximum_msat"`
	ShortChannelId string            `json:"short_channel_id,omitempty"`
	Delay          uint64            `json:"delay"`
}

type ClnListChanResp struct {
	Channels []ClnListChan `json:"channels,omitempty"`
}

type ClnFundsChan struct {
	PeerId         string `json:"peer_id"`
	Connected      bool   `json:"connected,omitempty"`
	ShortChannelId string `json:"short_channel_id"`
	State          string `json:"state"`
	Capacity       uint64 `json:"channel_total_sat"`
	OurAmount      uint64 `json:"channel_sat"`
	FundingTxId    string `json:"funding_tx_id"`
	FundingOutput  int    `json:"funding_output"`
}

type ClnFundsChanResp struct {
	Channels []ClnFundsChan `json:"channels,omitempty"`
}

type ClnSocketLightningApi struct {
	LightningApi
	Client *rpc.Client
}

type ClnListNodeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type ClnListNode struct {
	PubKey     string             `json:"nodeid"`
	Alias      string             `json:"alias,omitempty"`
	Color      string             `json:"color,omitempty"`
	Features   string             `json:"features,omitempty"`
	Addresses  []ClnListNodeAddr  `json:"addresses,omitempty"`
	LastUpdate *entities.JsonTime `json:"last_update,omitempty"`
}

type ClnListNodeResp struct {
	Nodes []ClnListNode `json:"nodes,omitempty"`
}

func ToLndChanId(id string) (uint64, error) {

	split := strings.Split(strings.ToLower(id), "x")
	if len(split) != 3 {
		return 0, fmt.Errorf("wrong channel id: %v", id)
	}

	blockId, err := strconv.ParseUint(split[0], 10, 64)
	if err != nil {
		return 0, err
	}

	txIdx, err := strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		return 0, err
	}

	outputIdx, err := strconv.ParseUint(split[2], 10, 64)
	if err != nil {
		return 0, err
	}

	result := (blockId&0xffffff)<<40 | (txIdx&0xffffff)<<16 | (outputIdx & 0xffff)

	return result, nil
}

func FromLndChanId(chanId uint64) string {
	blockId := int64((chanId & 0xffffff0000000000) >> 40)
	txIdx := int((chanId & 0x000000ffffff0000) >> 16)
	outputIdx := int(chanId & 0x000000000000ffff)

	return fmt.Sprintf("%dx%dx%d", blockId, txIdx, outputIdx)
}

func ConvertAmount(s string) uint64 {
	x := strings.ReplaceAll(s, "msat", "")
	ret, err := strconv.ParseUint(x, 10, 64)
	if err != nil {
		glog.Warningf("Could not convert: %v %v", s, err)
		return 0
	}

	return ret
}

func ConvertFeatures(features string) map[string]NodeFeatureApi {
	n := new(big.Int)

	n, ok := n.SetString(features, 16)
	if !ok {
		return nil
	}

	result := make(map[string]NodeFeatureApi)

	m := big.NewInt(0)
	zero := big.NewInt(0)
	two := big.NewInt(2)

	bit := 0
	for n.Cmp(zero) == 1 {
		n.DivMod(n, two, m)

		if m.Cmp(zero) != 1 {
			// Bit is not set
			bit++
			continue
		}

		result[fmt.Sprintf("%d", bit)] = NodeFeatureApi{
			Name:       "",
			IsKnown:    true,
			IsRequired: bit%2 == 0,
		}

		bit++
	}

	return result
}

func (l *ClnSocketLightningApi) GetInternalChannels(pubKey string) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.Client.Call("listchannels", []interface{}{nil, pubKey, nil}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelId == "" {
			continue
		}

		if _, ok := result[one.ShortChannelId]; !ok {
			result[one.ShortChannelId] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelId] = append(result[one.ShortChannelId], one)
	}

	err = l.Client.Call("listchannels", []interface{}{nil, nil, pubKey}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelId == "" {
			continue
		}

		if _, ok := result[one.ShortChannelId]; !ok {
			result[one.ShortChannelId] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelId] = append(result[one.ShortChannelId], one)
	}

	return result, nil
}

func (l *ClnSocketLightningApi) GetInternalChannelsAll() (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.Client.Call("listchannels", []interface{}{}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelId == "" {
			continue
		}

		if _, ok := result[one.ShortChannelId]; !ok {
			result[one.ShortChannelId] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelId] = append(result[one.ShortChannelId], one)
	}

	return result, nil
}

func SumCapacitySimple(channels []NodeChannelApi) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}

func SumCapacityExtended(channels []NodeChannelApiExtended) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}

func (l *ClnSocketLightningApi) Cleanup() {
	// do nothing
}

func (l *ClnSocketLightningApi) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error) {
	var reply ClnListNodeResp
	err := l.Client.Call("listnodes", []interface{}{}, &reply)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeApi, 0)

	for _, one := range reply.Nodes {
		nodes = append(nodes, *ConvertNodeInfo(one))
	}

	channels := make([]NodeChannelApi, 0)

	chans, err := l.GetInternalChannelsAll()
	if err != nil {
		return nil, err
	}

	for k, v := range chans {
		if len(v) < 1 {
			glog.Warningf("Bad data for %v", k)
			continue
		}

		lnd, err := ToLndChanId(k)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		ret, err := ConvertChannelInternal(v, lnd)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		if !unannounced && ret.Private {
			continue
		}

		channels = append(channels, ret.NodeChannelApi)
	}

	return &DescribeGraphApi{
		Nodes:    nodes,
		Channels: channels,
	}, nil
}

func ConvertChannelInternal(chans []ClnListChan, id uint64) (*NodeChannelApiExtended, error) {
	ret := &NodeChannelApiExtended{
		NodeChannelApi: NodeChannelApi{
			ChannelId: id,
			Capacity:  chans[0].Capacity,
		},
		Private: !chans[0].Public,
	}

	if chans[0].Source < chans[0].Destination {
		ret.Node1Pub = chans[0].Source
		ret.Node2Pub = chans[0].Destination
	} else {
		ret.Node1Pub = chans[0].Destination
		ret.Node2Pub = chans[0].Source
	}

	maxUpdate := time.Time{}
	for _, one := range chans {
		if time.Time(one.LastUpdate).After(maxUpdate) {
			maxUpdate = time.Time(one.LastUpdate)
		}

		if one.Source == ret.Node1Pub {
			ret.Node1Policy = &RoutingPolicyApi{
				TimeLockDelta: uint32(one.Delay),
				Disabled:      !one.Active,
				LastUpdate:    one.LastUpdate,
				BaseFee:       one.FeeBase,
				FeeRate:       one.FeePpm,
				MinHtlc:       ConvertAmount(one.MinHtlc),
				MaxHtlc:       ConvertAmount(one.MaxHtlc),
			}
		} else if one.Source == ret.Node2Pub {
			ret.Node2Policy = &RoutingPolicyApi{
				TimeLockDelta: uint32(one.Delay),
				Disabled:      !one.Active,
				LastUpdate:    one.LastUpdate,
				BaseFee:       one.FeeBase,
				FeeRate:       one.FeePpm,
				MinHtlc:       ConvertAmount(one.MinHtlc),
				MaxHtlc:       ConvertAmount(one.MaxHtlc),
			}
		} else {
			return nil, ErrInvalidResponse
		}
	}

	ret.LastUpdate = entities.JsonTime(maxUpdate)

	return ret, nil
}

func (l *ClnSocketLightningApi) GetChanInfo(ctx context.Context, chanId uint64) (*NodeChannelApi, error) {

	var listChanReply ClnListChanResp
	err := l.Client.Call("listchannels", []string{FromLndChanId(chanId)}, &listChanReply)
	if err != nil {
		return nil, err
	}

	// There can either be one or two channels
	if len(listChanReply.Channels) < 1 || len(listChanReply.Channels) > 2 {
		return nil, ErrNoChan
	}

	ret, err := ConvertChannelInternal(listChanReply.Channels, chanId)
	if err != nil {
		return nil, err
	}

	return &ret.NodeChannelApi, nil
}

func (l *ClnSocketLightningApi) GetChannels(ctx context.Context) (*ChannelsApi, error) {
	var fundsReply ClnFundsChanResp
	err := l.Client.Call("listfunds", []string{}, &fundsReply)
	if err != nil {
		return nil, err
	}

	channels := make([]ChannelApi, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {

		err = l.Client.Call("listchannels", []string{one.ShortChannelId}, &listChanReply)
		if err != nil {
			return nil, err
		}
		for _, two := range listChanReply.Channels {
			if two.Destination != one.PeerId {
				continue
			}

			lndchan, err := ToLndChanId(one.ShortChannelId)
			if err != nil {
				return nil, err
			}

			// TODO: we don't seems to have info about channel reserve, commit-fee, in-flight HTLCs etc.
			remoteBalance := uint64(0)
			if one.OurAmount <= two.Capacity {
				remoteBalance = two.Capacity - one.OurAmount
			}

			result := ChannelApi{
				Active:        two.Active,
				Private:       !two.Public,
				Capacity:      two.Capacity,
				RemotePubkey:  one.PeerId,
				ChanId:        lndchan,
				RemoteBalance: remoteBalance,
				LocalBalance:  one.OurAmount,

				// TODO: No data avilable
				CommitFee:             0,
				Initiator:             false,
				PendingHtlcs:          nil,
				TotalSatoshisSent:     0,
				TotalSatoshisReceived: 0,
				NumUpdates:            0,
			}

			channels = append(channels, result)
		}
	}

	return &ChannelsApi{Channels: channels}, nil
}

func (l *ClnSocketLightningApi) GetInfo(ctx context.Context) (*InfoApi, error) {
	var reply ClnInfo
	err := l.Client.Call("getinfo", []string{}, &reply)
	if err != nil {
		return nil, err
	}

	return &InfoApi{
		IdentityPubkey: reply.PubKey,
		Alias:          reply.Alias,
		Network:        reply.Network,
		Chain:          "mainnet", // assume mainnet
	}, nil
}

func ConvertNodeInfo(node ClnListNode) *DescribeGraphNodeApi {

	if node.LastUpdate == nil {
		glog.Warningf("Gossip message for node %s was not received yet", node.PubKey)
		return &DescribeGraphNodeApi{
			PubKey: node.PubKey,
		}
	}

	addresses := ConvertAddresses(node.Addresses)

	features := ConvertFeatures(node.Features)
	if features == nil {
		features = make(map[string]NodeFeatureApi)
	}

	return &DescribeGraphNodeApi{
		PubKey:     node.PubKey,
		Alias:      node.Alias,
		Color:      fmt.Sprintf("#%s", node.Color),
		LastUpdate: *node.LastUpdate,
		Addresses:  addresses,
		Features:   features,
	}
}

func ConvertAddresses(addr []ClnListNodeAddr) []NodeAddressApi {
	addresses := make([]NodeAddressApi, 0)
	for _, one := range addr {
		if one.Type == "ipv6" {
			addresses = append(addresses, NodeAddressApi{Network: "tcp", Addr: fmt.Sprintf("[%s]:%d", one.Address, one.Port)})
		} else if one.Type == "dns" || one.Type == "websocket" {
			// Ignore that stuff
			continue
		} else {
			addresses = append(addresses, NodeAddressApi{Network: "tcp", Addr: fmt.Sprintf("%s:%d", one.Address, one.Port)})
		}
	}
	return addresses
}

func (l *ClnSocketLightningApi) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoApi, error) {
	var reply ClnListNodeResp
	err := l.Client.Call("listnodes", []string{pubKey}, &reply)
	if err != nil {
		return nil, err
	}

	if len(reply.Nodes) != 1 {
		return nil, ErrNoNode
	}

	result := &NodeInfoApi{
		Node:          *ConvertNodeInfo(reply.Nodes[0]),
		Channels:      make([]NodeChannelApi, 0),
		NumChannels:   0,
		TotalCapacity: 0,
	}

	if !channels {
		return result, nil
	}

	chans, err := l.GetInternalChannels(pubKey)
	if err != nil {
		return result, err
	}

	for k, v := range chans {
		if len(v) < 1 {
			glog.Warningf("Bad data for %v", k)
			continue
		}

		lnd, err := ToLndChanId(k)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		ret, err := ConvertChannelInternal(v, lnd)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		result.Channels = append(result.Channels, ret.NodeChannelApi)
	}

	result.NumChannels = uint32(len(result.Channels))
	result.TotalCapacity = SumCapacitySimple(result.Channels)

	return result, nil

}

func (l *ClnSocketLightningApi) GetNodeInfoFull(ctx context.Context, channels bool, unannounced bool) (*NodeInfoApiExtended, error) {
	var reply ClnInfo
	err := l.Client.Call("getinfo", []string{}, &reply)
	if err != nil {
		return nil, err
	}

	node := DescribeGraphNodeApi{
		PubKey:    reply.PubKey,
		Alias:     reply.Alias,
		Color:     fmt.Sprintf("#%s", reply.Color),
		Addresses: ConvertAddresses(reply.Addresses),
		Features:  ConvertFeatures(reply.Features.Node),

		LastUpdate: entities.JsonTime(time.Now()),
	}

	result := &NodeInfoApiExtended{}
	result.Node = node
	result.Channels = make([]NodeChannelApiExtended, 0)
	result.NumChannels = 0
	result.TotalCapacity = 0

	if !channels {
		return result, nil
	}

	chans, err := l.GetInternalChannels(node.PubKey)
	if err != nil {
		return result, err
	}

	for k, v := range chans {
		if len(v) < 1 {
			glog.Warningf("Bad data for %v", k)
			continue
		}

		lnd, err := ToLndChanId(k)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		ret, err := ConvertChannelInternal(v, lnd)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		if !unannounced && ret.Private {
			continue
		}

		result.Channels = append(result.Channels, *ret)
	}

	result.NumChannels = uint32(len(result.Channels))
	result.TotalCapacity = SumCapacityExtended(result.Channels)

	return result, nil
}

// Compile time check for the interface
var _ LightingApiCalls = &ClnSocketLightningApi{}

// Usage "unix", "/home/ubuntu/.lightning/bitcoin/lightning-rpc"
func NewClnSocketLightningApiRaw(socketType string, address string) LightingApiCalls {
	client, err := rpc.Dial(socketType, address)
	if err != nil {
		glog.Warningf("Got error: %v", err)
		return nil
	}
	return &ClnSocketLightningApi{Client: client}
}

func NewClnSocketLightningApi(getData GetDataCall) LightingApiCalls {
	if getData == nil {
		return nil
	}
	data, err := getData()
	if data == nil || err != nil {
		return nil
	}

	return NewClnSocketLightningApiRaw("unix", data.Endpoint)
}
