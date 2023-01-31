package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
)

// Compile time check for the interface
var _ LightingAPICalls = &ClnSocketRawAPI{}

// Method names
const (
	LISTCHANNELS = "listchannels"
	LISTNODES    = "listnodes"
	LISTFUNDS    = "listfunds"
	GETINFO      = "getinfo"
	LISTFORWARDS = "listforwards"
	LISTINVOICES = "listinvoices"
	LISTPAYMENTS = "listsendpays"
)

// ClnSocketRawAPI struct
type ClnSocketRawAPI struct {
	API

	connection ClnConnectionAPI
}

// Cleanup - clean up API
func (l *ClnSocketRawAPI) Cleanup() {
	// do nothing
}

// DescribeGraph - DescribeGraph API call
func (l *ClnSocketRawAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	var reply ClnListNodeResp
	err := l.connection.Call(ctx, LISTNODES, []interface{}{}, &reply)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeAPI, 0)

	for _, one := range reply.Nodes {
		nodes = append(nodes, *ConvertNodeInfo(one))
	}

	channels := make([]NodeChannelAPI, 0)

	chans, err := l.GetInternalChannelsAll(ctx)
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

		ret, err := ConvertChannelInternal(v, lnd, "none")
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
func ConvertChannelInternal(chans []ClnListChan, id uint64, chanpoint string) (*NodeChannelAPIExtended, error) {
	ret := &NodeChannelAPIExtended{
		NodeChannelAPI: NodeChannelAPI{
			ChannelID: id,
			Capacity:  chans[0].Capacity,
			ChanPoint: chanpoint,
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
func (l *ClnSocketRawAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {

	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, LISTCHANNELS, []string{FromLndChanID(chanID)}, &listChanReply)
	if err != nil {
		return nil, err
	}

	// There can either be one or two channels
	if len(listChanReply.Channels) < 1 || len(listChanReply.Channels) > 2 {
		return nil, ErrNoChan
	}

	ret, err := ConvertChannelInternal(listChanReply.Channels, chanID, "none")
	if err != nil {
		return nil, err
	}

	return &ret.NodeChannelAPI, nil
}

// GetChannels - GetChannels API call
func (l *ClnSocketRawAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	var fundsReply ClnFundsChanResp
	err := l.connection.Call(ctx, LISTFUNDS, []string{}, &fundsReply)
	if err != nil {
		return nil, err
	}

	channels := make([]ChannelAPI, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {

		err = l.connection.Call(ctx, LISTCHANNELS, []string{one.ShortChannelID}, &listChanReply)
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

func (l *ClnSocketRawAPI) getMyChannels(ctx context.Context) ([]NodeChannelAPIExtended, error) {
	var fundsReply ClnFundsChanResp
	err := l.connection.Call(ctx, LISTFUNDS, []string{}, &fundsReply)
	if err != nil {
		return nil, err
	}

	channels := make([]NodeChannelAPIExtended, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {

		err = l.connection.Call(ctx, LISTCHANNELS, []string{one.ShortChannelID}, &listChanReply)
		if err != nil {
			return nil, err
		}

		if one.ShortChannelID == "" {
			continue
		}

		lnd, err := ToLndChanID(one.ShortChannelID)
		if err != nil {
			glog.Warningf("Could not convert %v", one.ShortChannelID)
			continue
		}

		ret, err := ConvertChannelInternal(listChanReply.Channels, lnd, fmt.Sprintf("%s:%d", one.FundingTxID, one.FundingOutput))
		if err != nil {
			return nil, err
		}

		channels = append(channels, *ret)
	}

	return channels, nil
}

// GetInfo - GetInfo API call
func (l *ClnSocketRawAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	var reply ClnInfo
	err := l.connection.Call(ctx, GETINFO, []string{}, &reply)
	if err != nil {
		return nil, err
	}

	syncedToGraph := reply.WarningLightningdSync == ""
	syncedToChain := reply.WarningBitcoindSync == ""

	return &InfoAPI{
		IdentityPubkey:  reply.PubKey,
		Alias:           reply.Alias,
		Network:         reply.Network,
		Chain:           "mainnet", // assume mainnet
		Version:         fmt.Sprintf("corelightning-%s", reply.Version),
		IsSyncedToGraph: syncedToGraph,
		IsSyncedToChain: syncedToChain,
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

// GetNodeInfo - API call
func (l *ClnSocketRawAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	var reply ClnListNodeResp
	err := l.connection.Call(ctx, LISTNODES, []string{pubKey}, &reply)
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

	chans, err := l.GetInternalChannels(ctx, pubKey)
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

		ret, err := ConvertChannelInternal(v, lnd, "none")
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

// GetNodeInfoFull - API call
func (l *ClnSocketRawAPI) GetNodeInfoFull(ctx context.Context, channels bool, unannounced bool) (*NodeInfoAPIExtended, error) {
	var reply ClnInfo
	err := l.connection.Call(ctx, GETINFO, []string{}, &reply)
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

	chans, err := l.getMyChannels(ctx)
	if err != nil {
		return nil, err
	}
	result.Channels = chans

	result.NumChannels = uint32(len(result.Channels))
	result.TotalCapacity = SumCapacityExtended(result.Channels)

	return result, nil
}

// SubscribeForwards - API call
func (l *ClnSocketRawAPI) SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16, maxErrors uint16) (<-chan []ForwardingEvent, <-chan ErrorData) {
	var reply ClnForwardEntries

	if batchSize == 0 {
		batchSize = l.API.GetDefaultBatchSize()
	}

	errors := 0
	errorChan := make(chan ErrorData, 1)
	outChan := make(chan []ForwardingEvent)

	go func() {

		batch := make([]ForwardingEvent, 0, batchSize)
		minTime := since

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Do nothing
			}

			err := l.connection.Call(ctx, LISTFORWARDS, []interface{}{}, &reply)
			if err != nil {
				glog.Warningf("Error getting forwards %v\n", err)
				if errors >= int(maxErrors) {
					errorChan <- ErrorData{Error: err, IsStillRunning: false}
					return
				}
				errorChan <- ErrorData{Error: err, IsStillRunning: true}
				continue
			}

			for _, one := range reply.Entries {
				select {
				case <-ctx.Done():
					return
				default:
					// Do nothing
				}

				if time.Time(one.ReceivedTime).Before(minTime) {
					continue
				}

				if one.Status == "settled" {
					continue
				}

				success := one.Status != "local_failed" && one.Status != "failed"

				batch = append(batch, ForwardingEvent{
					Timestamp:     time.Time(one.ReceivedTime),
					ChanIDIn:      stringToUint64(one.InChannel),
					ChanIDOut:     stringToUint64(one.OutChannel),
					AmountInMsat:  ConvertAmount(one.InMsat),
					AmountOutMsat: ConvertAmount(one.OutMsat),
					FeeMsat:       ConvertAmount(one.FeeMsat),
					IsSuccess:     success,
					FailureString: one.FailReason,
				})

				if len(batch) >= int(batchSize) {
					outChan <- batch
					batch = make([]ForwardingEvent, 0, batchSize)
				}
			}

			if len(batch) > 0 {
				outChan <- batch
				batch = make([]ForwardingEvent, 0, batchSize)
			}

			minTime = time.Time(reply.Entries[len(reply.Entries)-1].ReceivedTime)
		}
	}()

	return outChan, errorChan
}

func rawJSONIterator(ctx context.Context, reader io.Reader, name string, channel chan json.RawMessage) error {
	dec := json.NewDecoder(reader)

	// read open bracket
	t, err := dec.Token()
	if err != nil {
		return err
	}
	x, ok := t.(json.Delim)
	if !ok || x.String() != "{" {
		return fmt.Errorf("unexpected character")
	}

	// read array name
	t, err = dec.Token()
	if err != nil {
		return err
	}
	s, ok := t.(string)
	if !ok || s != name {
		return fmt.Errorf("unexpected name")
	}

	// read begin of array
	t, err = dec.Token()
	if err != nil {
		return err
	}
	x, ok = t.(json.Delim)
	if !ok || x.String() != "[" {
		return fmt.Errorf("unexpected character")
	}

	go func(ctx context.Context) {
		for dec.More() {
			// elements of array
			var m json.RawMessage

			select {
			case <-ctx.Done():
				return
			default:
				// Do nothing
			}
			err := dec.Decode(&m)
			if err != nil {
				return
			}

			channel <- m
		}

		close(channel)
	}(ctx)

	return nil
}

func getRaw[T ClnRawTimeItf](ctx context.Context, l *ClnSocketRawAPI, gettime T, method string, jsonArray string, pagination *RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	respPagination := &ResponseRawPagination{UseTimestamp: true}

	if pagination.BatchSize == 0 {
		pagination.BatchSize = uint64(l.GetDefaultBatchSize())
	}

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	reader, err := l.connection.StreamResponse(ctx, method, []interface{}{})
	if err != nil {
		return nil, respPagination, err
	}

	outchan := make(chan json.RawMessage, pagination.BatchSize)

	ret := make([]RawMessage, 0, pagination.BatchSize)

	err = rawJSONIterator(ctx, reader, jsonArray, outchan)
	if err != nil {
		return nil, respPagination, err
	}

	for {
		select {
		case <-ctx.Done():
			return ret, respPagination, nil
		case one, ok := <-outchan:
			if !ok {
				respPagination.FirstTime = minTime
				respPagination.LastTime = maxTime
				return ret, respPagination, nil
			}

			err = json.Unmarshal(one, &gettime)
			if err != nil {
				return nil, respPagination, err
			}

			t := time.Unix(0, int64(gettime.GetUnixTimeMs()*1e6))

			if pagination.From != nil && t.Before(*pagination.From) {
				continue
			}

			if t.Before(minTime) {
				minTime = t
			}
			if t.After(maxTime) {
				maxTime = t
			}

			m := RawMessage{
				Implementation: l.Name,
				Timestamp:      t,
				Message:        one,
			}

			ret = append(ret, m)

			if len(ret) >= int(pagination.BatchSize) {
				respPagination.FirstTime = minTime
				respPagination.LastTime = maxTime
				return ret, respPagination, nil
			}
		case <-time.After(1 * time.Second):
			respPagination.FirstTime = minTime
			respPagination.LastTime = maxTime
			return ret, respPagination, nil
		}

	}
}

// GetInvoicesRaw - API call
func (l *ClnSocketRawAPI) GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var (
		gettime ClnRawInvoiceTime
	)

	return getRaw(ctx, l, gettime, LISTINVOICES, "invoices", &pagination)
}

// GetPaymentsRaw - API call
func (l *ClnSocketRawAPI) GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var (
		gettime ClnRawPayTime
	)

	return getRaw(ctx, l, gettime, LISTPAYMENTS, "payments", &pagination)
}

// GetForwardsRaw - API call
func (l *ClnSocketRawAPI) GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var (
		gettime ClnRawForwardsTime
	)

	return getRaw(ctx, l, gettime, LISTFORWARDS, "forwards", &pagination)
}

// GetInvoices - API call
func (l *ClnSocketRawAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	panic("not implemented")
}

// GetPayments - API call
func (l *ClnSocketRawAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	panic("not implemented")
}

// GetAPIType - API call
func (l *ClnSocketRawAPI) GetAPIType() APIType {
	return ClnSocket
}

// GetInternalChannels - internal method to get channels
func (l *ClnSocketRawAPI) GetInternalChannels(ctx context.Context, pubKey string) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, LISTCHANNELS, []interface{}{nil, pubKey, nil}, &listChanReply)
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

	err = l.connection.Call(ctx, LISTCHANNELS, []interface{}{nil, nil, pubKey}, &listChanReply)
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
func (l *ClnSocketRawAPI) GetInternalChannelsAll(ctx context.Context) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, LISTCHANNELS, []interface{}{}, &listChanReply)
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
