package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
)

// Compile time check for the interface.
var _ LightingAPICalls = &ClnRawLightningAPI{}

// Method names.
const (
	ListChannels       = "listchannels"
	ListNodes          = "listnodes"
	ListFunds          = "listfunds"
	GetInfo            = "getinfo"
	ListForwards       = "listforwards"
	ListInvoices       = "listinvoices"
	ListSendPays       = "listsendpays"
	ListPays           = "listpays"
	Connect            = "connect"
	NewAddr            = "newaddr"
	Withdraw           = "withdraw"
	Pay                = "pay"
	InvoiceCmd         = "invoice"
	ListClosedChannels = "listclosedchannels"

	DefaultDuration = 1 * time.Hour
)

// ClnRawLightningAPI struct.
type ClnRawLightningAPI struct {
	API

	connection ClnConnectionAPI
}

// Cleanup - clean up API.
func (l *ClnRawLightningAPI) Cleanup() {
	l.connection.Cleanup()
}

// DescribeGraph - DescribeGraph API call.
func (l *ClnRawLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	var reply ClnListNodeResp
	err := l.connection.Call(ctx, ListNodes, []interface{}{}, &reply, DefaultDuration)
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

// ConvertChannelInternal - convert CLN channel to internal format.
func ConvertChannelInternal(chans []ClnListChan, id uint64, chanpoint string) (*NodeChannelAPIExtended, error) {
	if len(chans) == 0 {
		return nil, fmt.Errorf("invalid channels %d", id)
	}

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

// GetChanInfo - GetChanInfo API call.
func (l *ClnRawLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {
	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, ListChannels, []string{FromLndChanID(chanID)}, &listChanReply, DefaultDuration)
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

// GetChannels - GetChannels API call.
func (l *ClnRawLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	var fundsReply ClnFundsChanResp
	err := l.connection.Call(ctx, ListFunds, []string{}, &fundsReply, DefaultDuration)
	if err != nil {
		return nil, err
	}

	channels := make([]ChannelAPI, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {
		if one.ShortChannelID == "" {
			continue
		}
		err = l.connection.Call(ctx, ListChannels, []string{one.ShortChannelID}, &listChanReply, DefaultDuration)
		if err != nil {
			// skip bad response
			continue
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

				// TODO: No data available
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

func (l *ClnRawLightningAPI) getMyChannels(ctx context.Context) ([]NodeChannelAPIExtended, error) {
	var fundsReply ClnFundsChanResp
	err := l.connection.Call(ctx, ListFunds, []string{}, &fundsReply, DefaultDuration)
	if err != nil {
		return nil, err
	}

	channels := make([]NodeChannelAPIExtended, 0)

	var listChanReply ClnListChanResp

	for _, one := range fundsReply.Channels {
		if one.ShortChannelID == "" {
			continue
		}
		err = l.connection.Call(ctx, ListChannels, []string{one.ShortChannelID}, &listChanReply, DefaultDuration)
		if err != nil {
			continue
		}

		lnd, err := ToLndChanID(one.ShortChannelID)
		if err != nil {
			continue
		}

		ret, err := ConvertChannelInternal(listChanReply.Channels, lnd, fmt.Sprintf("%s:%d", one.FundingTxID, one.FundingOutput))
		if err != nil {
			continue
		}

		channels = append(channels, *ret)
	}

	return channels, nil
}

// GetInfo - GetInfo API call.
func (l *ClnRawLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	var reply ClnInfo
	err := l.connection.Call(ctx, GetInfo, []string{}, &reply, DefaultDuration)
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
		BlockHeight:     int(reply.Blockheight),
	}, nil
}

// ConvertNodeInfo - convert CLN to internal node representation.
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

// ConvertAddresses converts CLN addresses to interanl format.
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

// GetNodeInfo - API call.
func (l *ClnRawLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	var reply ClnListNodeResp
	err := l.connection.Call(ctx, ListNodes, []string{pubKey}, &reply, DefaultDuration)
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

// GetNodeInfoFull - API call.
func (l *ClnRawLightningAPI) GetNodeInfoFull(ctx context.Context, channels bool, unannounced bool) (*NodeInfoAPIExtended, error) {
	var reply ClnInfo
	err := l.connection.Call(ctx, GetInfo, []string{}, &reply, DefaultDuration)
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

// SubscribeForwards - API call.
func (l *ClnRawLightningAPI) SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16, maxErrors uint16) (<-chan []ForwardingEvent, <-chan ErrorData) {
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
			if ctx.Err() != nil {
				return
			}

			err := l.connection.Call(ctx, ListForwards, []interface{}{}, &reply, DefaultDuration)
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
				if ctx.Err() != nil {
					return
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

			if ctx.Err() != nil {
				return
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

func getRaw[T ClnRawTimeItf](ctx context.Context, l *ClnRawLightningAPI, gettime T, method string, jsonArray string, pagination *RawPagination) ([]RawMessage, *ResponseRawPagination, error) { //nolint:all
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

// GetInvoicesRaw - API call.
func (l *ClnRawLightningAPI) GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var gettime ClnRawInvoiceTime

	return getRaw(ctx, l, gettime, ListInvoices, "invoices", &pagination)
}

// GetPaymentsRaw - API call.
func (l *ClnRawLightningAPI) GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var gettime ClnRawPayTime

	return getRaw(ctx, l, gettime, ListSendPays, "payments", &pagination)
}

// GetForwardsRaw - API call.
func (l *ClnRawLightningAPI) GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	var gettime ClnRawForwardsTime

	return getRaw(ctx, l, gettime, ListForwards, "forwards", &pagination)
}

// GetInvoices - API call.
func (l *ClnRawLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	panic("not implemented")
}

// GetPayments - API call.
func (l *ClnRawLightningAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	panic("not implemented")
}

// GetAPIType - API call.
func (l *ClnRawLightningAPI) GetAPIType() APIType {
	return ClnSocket
}

// GetInternalChannels - internal method to get channels.
func (l *ClnRawLightningAPI) GetInternalChannels(ctx context.Context, pubKey string) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, ListChannels, []interface{}{nil, pubKey, nil}, &listChanReply, DefaultDuration)
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

	err = l.connection.Call(ctx, ListChannels, []interface{}{nil, nil, pubKey}, &listChanReply, DefaultDuration)
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

// GetInternalChannelsAll - internal method to get all channels.
func (l *ClnRawLightningAPI) GetInternalChannelsAll(ctx context.Context) (map[string][]ClnListChan, error) {
	result := make(map[string][]ClnListChan, 0)

	var listChanReply ClnListChanResp
	err := l.connection.Call(ctx, ListChannels, []interface{}{}, &listChanReply, DefaultDuration)
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

// ConnectPeer - API call.
func (l *ClnRawLightningAPI) ConnectPeer(ctx context.Context, id string) error {
	var resp ClnConnectResp
	err := l.connection.Call(ctx, Connect, []string{id}, &resp, DefaultDuration)
	if err != nil {
		return err
	}

	if len(resp.ID) == 0 {
		return fmt.Errorf("invalid response")
	}

	return nil
}

// GetOnChainAddress - API call.
func (l *ClnRawLightningAPI) GetOnChainAddress(ctx context.Context) (string, error) {
	var resp ClnNewAddrResp

	err := l.connection.Call(ctx, NewAddr, []interface{}{}, &resp, DefaultDuration)
	if err != nil {
		return "", err
	}

	if len(resp.Bech32) == 0 {
		return "", fmt.Errorf("invalid response")
	}

	return resp.Bech32, nil
}

// GetOnChainFunds - API call.
func (l *ClnRawLightningAPI) GetOnChainFunds(ctx context.Context) (*Funds, error) {
	const (
		Confirmed   = "confirmed"
		Unconfirmed = "uncomfirmed"
		Mili        = 1000
	)

	var fundsReply ClnFundsChainResp
	err := l.connection.Call(ctx, ListFunds, []string{}, &fundsReply, DefaultDuration)
	if err != nil {
		return nil, err
	}

	f := &Funds{}
	f.ConfirmedBalance = 0
	f.TotalBalance = 0
	f.LockedBalance = 0

	for _, one := range fundsReply.Outputs {
		amount := int64(ConvertAmount(one.AmountMsat))
		if !one.Reserved && one.Status == Confirmed {
			f.ConfirmedBalance += (amount / Mili)
			f.TotalBalance += (amount / Mili)
		} else if !one.Reserved && one.Status == Unconfirmed {
			f.TotalBalance += (amount / Mili)
		} else if one.Reserved && (one.Status == Unconfirmed || one.Status == Confirmed) {
			f.LockedBalance += (amount / Mili)
		}
	}

	return f, nil
}

// SendToOnChainAddress - API call.
func (l *ClnRawLightningAPI) SendToOnChainAddress(ctx context.Context, address string, sats int64, useUnconfirmed bool, urgency Urgency) (string, error) {
	var reply ClnWithdrawResp

	feerate := "normal"
	switch urgency {
	case Urgent:
		feerate = "urgent"
	case Normal:
		feerate = "normal"
	case Low:
		feerate = "slow" // slow not low, beware!
	}

	minConf := 1
	if useUnconfirmed {
		minConf = 0
	}

	err := l.connection.Call(ctx, Withdraw, []interface{}{address, sats, feerate, minConf}, &reply, DefaultDuration)
	if err != nil {
		return "", err
	}

	if len(reply.TxID) == 0 {
		return "", fmt.Errorf("invalid transaction id")
	}

	return reply.TxID, nil
}

// PayInvoice - API call.
func (l *ClnRawLightningAPI) PayInvoice(ctx context.Context, paymentRequest string, sats int64, outgoingChanIds []uint64) (*PaymentResp, error) {
	var (
		err   error
		reply ClnPayResp
	)

	exclusions, err := l.calculateExclusions(ctx, outgoingChanIds)
	if err != nil {
		return nil, err
	}

	// "The response will occur when the payment fails or succeeds. Once a payment has succeeded, calls to pay with the same bolt11 will succeed immediately.
	// Until retry_for seconds passes (default: 60), the command will keep finding routes and retrying the payment. However,
	// a payment may be delayed for up to maxdelay blocks by another node; clients should be prepared for this worst case.""
	// OR in layman terms this might just block for 2016 blocks (almost forever)

	timeoutSeconds := 10
	timeout := time.Duration((int64(timeoutSeconds) + 1) * int64(time.Second))
	if sats > 0 {
		err = l.connection.Call(ctx, Pay, []interface{}{paymentRequest, sats * 1000, nil, nil, nil, timeoutSeconds, nil, nil, nil, exclusions}, &reply, timeout)
	} else {
		err = l.connection.Call(ctx, Pay, []interface{}{paymentRequest, nil, nil, nil, nil, nil, nil, timeoutSeconds, nil, exclusions}, &reply, timeout)
	}

	if err != nil {
		return nil, err
	}

	r := &PaymentResp{}
	if reply.Status == "complete" {
		r.Preimage = reply.PaymentPreimage
		r.Hash = reply.PaymentHash
		r.Status = Success
	} else if reply.Status == "pending" {
		r.Hash = reply.PaymentHash
		r.Preimage = ""
		r.Status = Pending
	} else {
		return nil, fmt.Errorf("payment failed")
	}

	return r, nil
}

// calculateExclusions is used since CLN can just exclude channels, so we tell it which channels we want and method calculates the "inverse" of it
func (l *ClnRawLightningAPI) calculateExclusions(ctx context.Context, outgoingChanIds []uint64) ([]string, error) {
	result := make([]string, 0)

	if outgoingChanIds == nil || len(outgoingChanIds) == 0 {
		return nil, nil
	}

	var info ClnInfo
	err := l.connection.Call(ctx, GetInfo, []string{}, &info, DefaultDuration)
	if err != nil {
		return nil, err
	}

	chans, err := l.getMyChannels(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[uint64]struct{})
	for _, id := range outgoingChanIds {
		m[id] = struct{}{}
	}

	for _, one := range chans {
		_, ok := m[one.ChannelID]
		if ok {
			continue
		}

		// not an outgoing channel, create exclusion

		id := FromLndChanID(one.ChannelID)

		peer := ""
		if one.Node1Pub == info.PubKey {
			peer = one.Node2Pub
		} else if one.Node2Pub == info.PubKey {
			peer = one.Node1Pub
		} else {
			glog.Warningf("Channel %d is not mine?", one.ChannelID)
			continue
		}

		// direction (u32): 0 if this channel is traversed from lesser to greater id, otherwise 1
		direction := 0
		if info.PubKey > peer {
			direction = 1
		}

		result = append(result, fmt.Sprintf("%s/%d", id, direction))
	}

	return result, nil
}

// GetPaymentStatus - API call.
func (l *ClnRawLightningAPI) GetPaymentStatus(ctx context.Context, paymentHash string) (*PaymentResp, error) {
	var reply ClnPaymentEntries
	err := l.connection.Call(ctx, ListPays, []interface{}{nil, paymentHash}, &reply, DefaultDuration)
	if err != nil {
		return nil, err
	}

	if len(reply.Entries) != 1 {
		return nil, fmt.Errorf("invalid response")
	}

	hash := reply.Entries[0].PaymentHash

	if hash != paymentHash {
		return nil, fmt.Errorf("invalid response")
	}

	switch reply.Entries[0].Status {
	case "failed":
		return &PaymentResp{
			Status:   Failed,
			Preimage: "",
			Hash:     hash,
		}, nil
	case "complete":
		return &PaymentResp{
			Status:   Success,
			Preimage: reply.Entries[0].PaymentPreimage,
			Hash:     hash,
		}, nil
	default:
		return &PaymentResp{
			Status:   Pending,
			Preimage: "",
			Hash:     hash,
		}, nil
	}
}

// CreateInvoice - API call.
func (l *ClnRawLightningAPI) CreateInvoice(ctx context.Context, sats int64, preimage string, memo string, expiry time.Duration) (*InvoiceResp, error) {
	var (
		err   error
		reply ClnInvoiceResp
	)

	duration := int(expiry.Seconds())

	if sats > 0 {
		if preimage != "" {
			err = l.connection.Call(ctx, InvoiceCmd, []interface{}{sats * 1000, "", memo, duration, nil, preimage}, &reply, DefaultDuration)
		} else {
			err = l.connection.Call(ctx, InvoiceCmd, []interface{}{sats * 1000, "", memo, duration, nil, nil}, &reply, DefaultDuration)
		}
	} else {
		if preimage != "" {
			err = l.connection.Call(ctx, InvoiceCmd, []interface{}{"any", "", memo, duration, nil, preimage}, &reply, DefaultDuration)
		} else {
			err = l.connection.Call(ctx, InvoiceCmd, []interface{}{"any", "", memo, duration, nil, nil}, &reply, DefaultDuration)
		}
	}

	if err != nil {
		return nil, err
	}

	if reply.Bolt11 == "" {
		return nil, fmt.Errorf("missing bolt11")
	}

	return &InvoiceResp{
		PaymentRequest: reply.Bolt11,
		Hash:           reply.PaymentHash,
	}, nil
}

// IsInvoicePaid - API call.
func (l *ClnRawLightningAPI) IsInvoicePaid(ctx context.Context, paymentHash string) (bool, error) {
	var reply ClnInvoiceEntries
	err := l.connection.Call(ctx, ListInvoices, []interface{}{nil, nil, paymentHash}, &reply, DefaultDuration)
	if err != nil {
		return false, err
	}

	if len(reply.Entries) != 1 {
		return false, fmt.Errorf("invalid response")
	}

	if reply.Entries[0].PaymentHash != paymentHash {
		return false, fmt.Errorf("invalid payment hash")
	}

	return reply.Entries[0].Status == "paid", nil
}

func (l *ClnRawLightningAPI) convertInitiator(initiator string) CommonInitiator {
	switch initiator {
	case "local":
		return Local
	case "remote":
		return Remote
	default:
		return Unknown
	}
}

// GetChannelCloseInfoLegacy - API call.
func (l *ClnRawLightningAPI) GetChannelCloseInfoLegacy(ctx context.Context, chanIDs []uint64) ([]CloseInfo, error) {
	// We could use listpeerchannels but that was also just recently introduced, thus use listfunds
	var (
		reply ClnFundsChanResp
		ids   []uint64
	)

	err := l.connection.Call(ctx, ListFunds, []interface{}{}, &reply, DefaultDuration)
	if err != nil {
		return nil, err
	}

	lookup := make(map[uint64]ClnFundsChan)

	if chanIDs != nil {
		ids = chanIDs
	} else {
		ids = make([]uint64, 0)
	}

	for _, channel := range reply.Channels {
		if channel.ShortChannelID == "" {
			continue
		}
		if channel.State == "CHANNELD_NORMAL" {
			continue
		}

		id, err := ToLndChanID(channel.ShortChannelID)
		if err != nil {
			continue
		}
		lookup[id] = channel
		if chanIDs == nil {
			ids = append(chanIDs, id)
		}
	}

	ret := make([]CloseInfo, 0)

	for _, id := range ids {
		if c, ok := lookup[id]; ok {

			typ := CooperativeType
			switch c.State {
			case "FUNDING_SPEND_SEEN":
			case "ONCHAIN":
				typ = ForceType
			}
			ret = append(ret, CloseInfo{
				ChanID:    id,
				Opener:    Unknown,
				Closer:    Unknown,
				CloseType: typ,
			})
		} else {
			ret = append(ret, UnknownCloseInfo)
		}
	}

	return ret, nil
}

// GetChannelCloseInfo - API call.
func (l *ClnRawLightningAPI) GetChannelCloseInfo(ctx context.Context, chanIDs []uint64) ([]CloseInfo, error) {
	var (
		reply ClnClosedChannelEntires
		ids   []uint64
	)

	err := l.connection.Call(ctx, ListClosedChannels, []interface{}{}, &reply, DefaultDuration)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown command") {
			return l.GetChannelCloseInfoLegacy(ctx, chanIDs)
		} else {
			return nil, err
		}
	}

	lookup := make(map[uint64]ClnClosedChannelEntry)

	if chanIDs != nil {
		ids = chanIDs
	} else {
		ids = make([]uint64, 0)
	}

	for _, channel := range reply.Entries {
		if channel.ShortChannelID == "" {
			continue
		}

		id, err := ToLndChanID(channel.ShortChannelID)
		if err != nil {
			continue
		}
		lookup[id] = channel
		if chanIDs == nil {
			ids = append(chanIDs, id)
		}
	}

	ret := make([]CloseInfo, 0)

	for _, id := range ids {
		if c, ok := lookup[id]; ok {
			typ := UnknownType

			switch c.CloseCause {
			case "local":
			case "user":
			case "remote":
				typ = CooperativeType
			case "protocol":
			case "onchain":
				typ = ForceType
			}
			ret = append(ret, CloseInfo{
				ChanID:    id,
				Opener:    l.convertInitiator(c.Opener),
				Closer:    l.convertInitiator(c.Closer),
				CloseType: typ,
			})
		} else {
			ret = append(ret, UnknownCloseInfo)
		}
	}

	return ret, nil
}
