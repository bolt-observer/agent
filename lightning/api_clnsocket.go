package lightning

import (
	"context"
	"fmt"
	"os"
	"time"

	r "net/rpc"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

// Compile time check for the interface
var _ LightingAPICalls = &ClnSocketLightningAPI{}

// Method names
const (
	LISTCHANNELS = "listchannels"
	LISTNODES    = "listnodes"
	LISTFUNDS    = "listfunds"
	GETINFO      = "getinfo"
)

// NewClnSocketLightningAPIRaw gets a new API - usage "unix", "/home/ubuntu/.lightning/bitcoin/lightning-rpc"
func NewClnSocketLightningAPIRaw(socketType string, address string) LightingAPICalls {
	client, err := rpc.Dial(socketType, address)
	if err != nil {
		glog.Warningf("Got error: %v", err)
		return nil
	}
	return &ClnSocketLightningAPI{Client: client, Timeout: time.Second * 30}
}

// NewClnSocketLightningAPI return a new lightning API
func NewClnSocketLightningAPI(getData GetDataCall) LightingAPICalls {
	if getData == nil {
		return nil
	}
	data, err := getData()
	if data == nil || err != nil {
		return nil
	}

	return NewClnSocketLightningAPIRaw("unix", data.Endpoint)
}

// Cleanup - clean up API
func (l *ClnSocketLightningAPI) Cleanup() {
	// do nothing
}

// DescribeGraph - DescribeGraph API call
func (l *ClnSocketLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	var reply ClnListNodeResp
	err := l.CallWithTimeout(LISTNODES, []interface{}{}, &reply)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeAPI, 0)

	for _, one := range reply.Nodes {
		nodes = append(nodes, *ConvertNodeInfo(one))
	}

	channels := make([]NodeChannelAPI, 0)

	chans, err := l.GetInternalChannelsAll()
	if err != nil {
		return nil, err
	}

	for k, v := range chans {
		if len(v) < 1 {
			glog.Warningf("Bad data for %v", k)
			continue
		}

		lnd, err := ToLndChanID(k)
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

		channels = append(channels, ret.NodeChannelAPI)
	}

	return &DescribeGraphAPI{
		Nodes:    nodes,
		Channels: channels,
	}, nil
}

// ConvertChannelInternal - convert CLN channel to internal format
func ConvertChannelInternal(chans []ClnListChan, id uint64) (*NodeChannelAPIExtended, error) {
	ret := &NodeChannelAPIExtended{
		NodeChannelAPI: NodeChannelAPI{
			ChannelID: id,
			Capacity:  chans[0].Capacity,
			// TODO: we don't have that
			ChanPoint: "none",
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
			ret.Node1Policy = &RoutingPolicyAPI{
				TimeLockDelta: uint32(one.Delay),
				Disabled:      !one.Active,
				LastUpdate:    one.LastUpdate,
				BaseFee:       one.FeeBase,
				FeeRate:       one.FeePpm,
				MinHtlc:       ConvertAmount(one.MinHtlc),
				MaxHtlc:       ConvertAmount(one.MaxHtlc),
			}
		} else if one.Source == ret.Node2Pub {
			ret.Node2Policy = &RoutingPolicyAPI{
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

// GetChanInfo - GetChanInfo API call
func (l *ClnSocketLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {

	var listChanReply ClnListChanResp
	err := l.CallWithTimeout(LISTCHANNELS, []string{FromLndChanID(chanID)}, &listChanReply)
	if err != nil {
		return nil, err
	}

	// There can either be one or two channels
	if len(listChanReply.Channels) < 1 || len(listChanReply.Channels) > 2 {
		return nil, ErrNoChan
	}

	ret, err := ConvertChannelInternal(listChanReply.Channels, chanID)
	if err != nil {
		return nil, err
	}

	return &ret.NodeChannelAPI, nil
}

// GetChannels - GetChannels API call
func (l *ClnSocketLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	var fundsReply ClnFundsChanResp
	err := l.CallWithTimeout(LISTFUNDS, []string{}, &fundsReply)
	if err != nil {
		return nil, err
	}

	channels := make([]ChannelAPI, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {

		err = l.CallWithTimeout(LISTCHANNELS, []string{one.ShortChannelID}, &listChanReply)
		if err != nil {
			return nil, err
		}
		for _, two := range listChanReply.Channels {
			if two.Destination != one.PeerID {
				continue
			}

			lndchan, err := ToLndChanID(one.ShortChannelID)
			if err != nil {
				return nil, err
			}

			// TODO: we don't seems to have info about channel reserve, commit-fee, in-flight HTLCs etc.
			remoteBalance := uint64(0)
			if one.OurAmount <= two.Capacity {
				remoteBalance = two.Capacity - one.OurAmount
			}

			result := ChannelAPI{
				Active:        two.Active,
				Private:       !two.Public,
				Capacity:      two.Capacity,
				RemotePubkey:  one.PeerID,
				ChanID:        lndchan,
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

	return &ChannelsAPI{Channels: channels}, nil
}

// GetInfo - GetInfo API call
func (l *ClnSocketLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	var reply ClnInfo
	err := l.CallWithTimeout(GETINFO, []string{}, &reply)
	if err != nil {
		return nil, err
	}

	return &InfoAPI{
		IdentityPubkey: reply.PubKey,
		Alias:          reply.Alias,
		Network:        reply.Network,
		Chain:          "mainnet", // assume mainnet
	}, nil
}

// ConvertNodeInfo - convert CLN to internal node representation
func ConvertNodeInfo(node ClnListNode) *DescribeGraphNodeAPI {
	if node.LastUpdate == nil {
		glog.Warningf("Gossip message for node %s was not received yet", node.PubKey)
		return &DescribeGraphNodeAPI{
			PubKey: node.PubKey,
		}
	}

	addresses := ConvertAddresses(node.Addresses)

	features := ConvertFeatures(node.Features)
	if features == nil {
		features = make(map[string]NodeFeatureAPI)
	}

	return &DescribeGraphNodeAPI{
		PubKey:     node.PubKey,
		Alias:      node.Alias,
		Color:      fmt.Sprintf("#%s", node.Color),
		LastUpdate: *node.LastUpdate,
		Addresses:  addresses,
		Features:   features,
	}
}

// ConvertAddresses converts CLN addresses to interanl format
func ConvertAddresses(addr []ClnListNodeAddr) []NodeAddressAPI {
	addresses := make([]NodeAddressAPI, 0)
	for _, one := range addr {
		if one.Type == "ipv6" {
			addresses = append(addresses, NodeAddressAPI{Network: "tcp", Addr: fmt.Sprintf("[%s]:%d", one.Address, one.Port)})
		} else if one.Type == "dns" || one.Type == "websocket" {
			// Ignore that stuff
			continue
		} else {
			addresses = append(addresses, NodeAddressAPI{Network: "tcp", Addr: fmt.Sprintf("%s:%d", one.Address, one.Port)})
		}
	}
	return addresses
}

// GetNodeInfo - GetNodeInfo API call
func (l *ClnSocketLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	var reply ClnListNodeResp
	err := l.CallWithTimeout(LISTNODES, []string{pubKey}, &reply)
	if err != nil {
		return nil, err
	}

	if len(reply.Nodes) != 1 {
		return nil, ErrNoNode
	}

	result := &NodeInfoAPI{
		Node:          *ConvertNodeInfo(reply.Nodes[0]),
		Channels:      make([]NodeChannelAPI, 0),
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

		lnd, err := ToLndChanID(k)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		ret, err := ConvertChannelInternal(v, lnd)
		if err != nil {
			glog.Warningf("Could not convert %v", k)
			continue
		}

		result.Channels = append(result.Channels, ret.NodeChannelAPI)
	}

	result.NumChannels = uint32(len(result.Channels))
	result.TotalCapacity = SumCapacitySimple(result.Channels)

	return result, nil

}

// GetNodeInfoFull - GetNodeInfoFull API call
func (l *ClnSocketLightningAPI) GetNodeInfoFull(ctx context.Context, channels bool, unannounced bool) (*NodeInfoAPIExtended, error) {
	var reply ClnInfo
	err := l.CallWithTimeout(GETINFO, []string{}, &reply)
	if err != nil {
		return nil, err
	}

	node := DescribeGraphNodeAPI{
		PubKey:    reply.PubKey,
		Alias:     reply.Alias,
		Color:     fmt.Sprintf("#%s", reply.Color),
		Addresses: ConvertAddresses(reply.Addresses),
		Features:  ConvertFeatures(reply.Features.Node),

		LastUpdate: entities.JsonTime(time.Now()),
	}

	result := &NodeInfoAPIExtended{}
	result.Node = node
	result.Channels = make([]NodeChannelAPIExtended, 0)
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

		lnd, err := ToLndChanID(k)
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

// GetForwardingHistory - GetForwardingHistory API call
func (l *ClnSocketLightningAPI) GetForwardingHistory(ctx context.Context, pagination Pagination) (*ForwardingHistoryResponse, error) {
	panic("not implemented")
}

// GetInvoices - GetInvoices API call
func (l *ClnSocketLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	panic("not implemented")
}

// GetInternalChannels - internal method to get channels
func (l *ClnSocketLightningAPI) GetInternalChannels(pubKey string) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.CallWithTimeout(LISTCHANNELS, []interface{}{nil, pubKey, nil}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelID == "" {
			continue
		}

		if _, ok := result[one.ShortChannelID]; !ok {
			result[one.ShortChannelID] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelID] = append(result[one.ShortChannelID], one)
	}

	err = l.CallWithTimeout(LISTCHANNELS, []interface{}{nil, nil, pubKey}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelID == "" {
			continue
		}

		if _, ok := result[one.ShortChannelID]; !ok {
			result[one.ShortChannelID] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelID] = append(result[one.ShortChannelID], one)
	}

	return result, nil
}

// GetInternalChannelsAll - internal method to get all channels
func (l *ClnSocketLightningAPI) GetInternalChannelsAll() (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.CallWithTimeout(LISTCHANNELS, []interface{}{}, &listChanReply)
	if err != nil {
		return nil, err
	}

	for _, one := range listChanReply.Channels {
		if one.ShortChannelID == "" {
			continue
		}

		if _, ok := result[one.ShortChannelID]; !ok {
			result[one.ShortChannelID] = make([]ClnListChan, 0)
		}

		result[one.ShortChannelID] = append(result[one.ShortChannelID], one)
	}

	return result, nil
}

// CallWithTimeout - helper to call rpc method with a timeout
func (l *ClnSocketLightningAPI) CallWithTimeout(serviceMethod string, args any, reply any) error {
	c := make(chan *r.Call, 1)
	go func() { l.Client.Go(serviceMethod, args, reply, c) }()
	select {
	case call := <-c:
		return call.Error
	case <-time.After(l.Timeout):
		return os.ErrDeadlineExceeded
	}
}
