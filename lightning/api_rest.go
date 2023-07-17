package lightning

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/golang/glog"
	"github.com/lightningnetwork/lnd/lnrpc"
)

// LndRestLightningAPI struct.
type LndRestLightningAPI struct {
	Request   *http.Request
	Transport *http.Transport
	HTTPAPI   *HTTPAPI

	API
}

// Compile time check for the interface.
var _ LightingAPICalls = &LndRestLightningAPI{}

// NewLndRestLightningAPI constructs new lightning API.
func NewLndRestLightningAPI(getData GetDataCall) LightingAPICalls {
	api := NewHTTPAPI()

	request, transport, err := api.GetHTTPRequest(getData)
	if err != nil {
		glog.Warningf("Failed to get client: %v", err)
		return nil
	}

	api.SetTransport(transport)

	ret := &LndRestLightningAPI{
		Request:   request,
		Transport: transport,
		HTTPAPI:   api,
		API:       API{GetNodeInfoFullThreshUseDescribeGraph: 500},
	}

	ret.Name = "lndrest"

	return ret
}

// GetInfo - GetInfo API.
func (l *LndRestLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetInfo(ctx, l.Request)
	if err != nil {
		return nil, err
	}

	ret := &InfoAPI{
		Alias:           resp.Alias,
		IdentityPubkey:  resp.IdentityPubkey,
		Chain:           resp.Chains[0].Chain,
		Network:         resp.Chains[0].Network,
		Version:         fmt.Sprintf("lnd-%s", resp.Version),
		IsSyncedToGraph: resp.SyncedToGraph,
		IsSyncedToChain: resp.SyncedToChain,
		BlockHeight:     int(resp.BlockHeight),
	}

	return ret, err
}

// Cleanup - clean up.
func (l *LndRestLightningAPI) Cleanup() {
	// Nothing to do here
}

func stringToUint64(str string) uint64 {
	if str == "" {
		return 0
	}

	ret, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

func stringToInt64(str string) int64 {
	if str == "" {
		return 0
	}

	ret, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}

	return ret
}

// GetChannels - GetChannels API.
func (l *LndRestLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetChannels(ctx, l.Request)
	if err != nil {
		return nil, err
	}

	chans := make([]ChannelAPI, 0)
	for _, channel := range resp.Channels {
		htlcs := make([]HtlcAPI, 0)
		for _, h := range channel.PendingHtlcs {
			htlcs = append(htlcs, HtlcAPI{
				Amount:              stringToUint64(h.Amount),
				Incoming:            h.Incoming,
				ForwardingChannel:   stringToUint64(h.ForwardingChannel),
				ForwardingHtlcIndex: stringToUint64(h.ForwardingHtlcIndex),
			})
		}

		chans = append(chans, ChannelAPI{
			Private:               channel.Private,
			Active:                channel.Active,
			RemotePubkey:          channel.RemotePubkey,
			ChanID:                stringToUint64(channel.ChanID),
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

	ret := &ChannelsAPI{
		Channels: chans,
	}

	return ret, nil
}

func toPolicyWeb(policy *RoutingPolicyOverride) *RoutingPolicyAPI {
	if policy == nil {
		return nil
	}

	return &RoutingPolicyAPI{
		TimeLockDelta: policy.TimeLockDelta,
		MinHtlc:       stringToUint64(policy.MinHtlc),
		BaseFee:       stringToUint64(policy.FeeBaseMsat),
		FeeRate:       stringToUint64(policy.FeeRateMilliMsat),
		Disabled:      policy.Disabled,
		LastUpdate:    entities.JsonTime(time.Unix(int64(policy.LastUpdate), 0)),
		MaxHtlc:       stringToUint64(policy.MaxHtlcMsat),
	}
}

// DescribeGraph - DescribeGraph API.
func (l *LndRestLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetGraph(ctx, l.Request, unannounced)
	if err != nil {
		return nil, err
	}

	nodes := make([]DescribeGraphNodeAPI, 0)

	for _, node := range resp.GraphNodeOverride {
		nodes = append(nodes, l.convertNode(node))
	}

	channels := make([]NodeChannelAPI, 0)

	for _, edge := range resp.GraphEdgesOverride {
		channels = append(channels, l.convertChan(edge))
	}

	ret := &DescribeGraphAPI{
		Nodes:    nodes,
		Channels: channels,
	}

	return ret, nil
}

func (l *LndRestLightningAPI) convertNode(node *GraphNodeOverride) DescribeGraphNodeAPI {
	addresses := make([]NodeAddressAPI, 0)
	for _, addr := range node.Addresses {
		addresses = append(addresses, NodeAddressAPI{Addr: addr.Addr, Network: addr.Network})
	}

	features := make(map[string]NodeFeatureAPI)
	for id, feat := range node.Features {
		features[fmt.Sprintf("%d", id)] = NodeFeatureAPI{Name: feat.Name, IsRequired: feat.IsRequired, IsKnown: feat.IsKnown}
	}

	return DescribeGraphNodeAPI{
		PubKey: node.PubKey, Alias: node.Alias, Color: node.Color, Features: features, Addresses: addresses,
		LastUpdate: entities.JsonTime(time.Unix(int64(node.LastUpdate), 0)),
	}
}

func (l *LndRestLightningAPI) convertChan(edge *GraphEdgeOverride) NodeChannelAPI {
	return NodeChannelAPI{
		ChannelID:   stringToUint64(edge.ChannelID),
		ChanPoint:   edge.ChanPoint,
		Node1Pub:    edge.Node1Pub,
		Node2Pub:    edge.Node2Pub,
		Capacity:    stringToUint64(edge.Capacity),
		Node1Policy: toPolicyWeb(edge.Node1Policy),
		Node2Policy: toPolicyWeb(edge.Node2Policy),
		LastUpdate:  entities.JsonTime(time.Unix(int64(edge.LastUpdate), 0)),
	}
}

// GetNodeInfo - GetNodeInfo API.
func (l *LndRestLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetNodeInfo(ctx, l.Request, pubKey, channels)
	if err != nil {
		return nil, err
	}

	ch := make([]NodeChannelAPI, 0)

	for _, edge := range resp.Channels {
		ch = append(ch, l.convertChan(edge))
	}

	ret := &NodeInfoAPI{Node: l.convertNode(resp.Node), Channels: ch, NumChannels: uint32(resp.NumChannels), TotalCapacity: stringToUint64(resp.TotalCapacity)}

	return ret, nil
}

// GetChanInfo - GetChanInfo API.
func (l *LndRestLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {
	resp, err := l.HTTPAPI.HTTPGetChanInfo(ctx, l.Request, chanID)
	if err != nil {
		return nil, err
	}
	ret := l.convertChan(resp)
	return &ret, nil
}

// SubscribeForwards - API call.
func (l *LndRestLightningAPI) SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16, maxErrors uint16) (<-chan []ForwardingEvent, <-chan ErrorData) {
	panic("not implemented")
}

// GetInvoicesRaw - API call.
func (l *LndRestLightningAPI) GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ListInvoiceRequestOverride{
		NumMaxInvoices: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset:    fmt.Sprintf("%d", pagination.Offset),
	}
	respPagination := &ResponseRawPagination{UseTimestamp: false}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	if pagination.From != nil || pagination.To != nil {
		return nil, respPagination, fmt.Errorf("from and to are not yet supported")
	}
	*/

	if pagination.Reversed {
		param.Reversed = true
	}

	if pendingOnly {
		param.PendingOnly = pendingOnly
	}

	resp, err := l.HTTPAPI.HTTPListInvoices(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	respPagination.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	ret := make([]RawMessage, 0, len(resp.Invoices))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, invoice := range resp.Invoices {
		t := time.Unix(stringToInt64(invoice.CreationDate), 0)
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(invoice)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetPaymentsRaw - API call.
func (l *LndRestLightningAPI) GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ListPaymentsRequestOverride{
		MaxPayments: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset: fmt.Sprintf("%d", pagination.Offset),
	}
	respPagination := &ResponseRawPagination{UseTimestamp: false}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	if pagination.From != nil || pagination.To != nil {
		return nil, respPagination, fmt.Errorf("from and to are not yet supported")
	}
	*/

	if pagination.Reversed {
		param.Reversed = true
	}

	if includeIncomplete {
		param.IncludeIncomplete = includeIncomplete
	}

	resp, err := l.HTTPAPI.HTTPListPayments(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	respPagination.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	ret := make([]RawMessage, 0, len(resp.Payments))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, payment := range resp.Payments {
		t := time.Unix(stringToInt64(payment.CreationDate), 0)
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(payment)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetForwardsRaw - API call.
func (l *LndRestLightningAPI) GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	param := &ForwardingHistoryRequestOverride{}

	param.NumMaxEvents = uint32(pagination.BatchSize)
	param.IndexOffset = uint32(pagination.Offset)

	if pagination.From != nil {
		param.StartTime = fmt.Sprintf("%d", pagination.From.Unix())
	}

	if pagination.To != nil {
		param.EndTime = fmt.Sprintf("%d", pagination.To.Unix())
	}

	respPagination := &ResponseRawPagination{UseTimestamp: false}

	resp, err := l.HTTPAPI.HTTPForwardEvents(ctx, l.Request, param)
	if err != nil {
		return nil, respPagination, err
	}

	respPagination.LastOffsetIndex = uint64(resp.LastOffsetIndex)
	respPagination.FirstOffsetIndex = 0

	ret := make([]RawMessage, 0, len(resp.ForwardingEvents))

	minTime := time.Unix(1<<63-1, 0)
	maxTime := time.Unix(0, 0)

	for _, forward := range resp.ForwardingEvents {
		t := time.Unix(0, stringToInt64(forward.TimestampNs))
		if t.Before(minTime) {
			minTime = t
		}
		if t.After(maxTime) {
			maxTime = t
		}

		m := RawMessage{
			Implementation: l.Name,
			Timestamp:      t,
		}
		m.Message, err = json.Marshal(forward)
		if err != nil {
			return nil, respPagination, err
		}

		ret = append(ret, m)
	}

	respPagination.FirstTime = minTime
	respPagination.LastTime = maxTime

	return ret, respPagination, nil
}

// GetInvoices API.
func (l *LndRestLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	param := &ListInvoiceRequestOverride{
		NumMaxInvoices: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset:    fmt.Sprintf("%d", pagination.Offset),
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		param.Reversed = true
	}

	if pendingOnly {
		param.PendingOnly = pendingOnly
	}

	resp, err := l.HTTPAPI.HTTPListInvoices(ctx, l.Request, param)
	if err != nil {
		return nil, err
	}

	ret := &InvoicesResponse{
		Invoices: make([]Invoice, 0, len(resp.Invoices)),
	}

	ret.LastOffsetIndex = stringToUint64(resp.LastIndexOffset)
	ret.FirstOffsetIndex = stringToUint64(resp.FirstIndexOffset)

	for _, invoice := range resp.Invoices {
		ret.Invoices = append(ret.Invoices, Invoice{
			Memo:            invoice.Memo,
			ValueMsat:       stringToInt64(invoice.ValueMsat),
			PaidMsat:        stringToInt64(invoice.AmtPaidMsat),
			CreationDate:    time.Unix(stringToInt64(invoice.CreationDate), 0),
			SettleDate:      time.Unix(stringToInt64(invoice.SettleDate), 0),
			PaymentRequest:  invoice.PaymentRequest,
			DescriptionHash: string(invoice.DescriptionHash),
			Expiry:          stringToInt64(invoice.Expiry),
			FallbackAddr:    invoice.FallbackAddr,
			CltvExpiry:      stringToUint64(invoice.CltvExpiry),
			Private:         invoice.Private,
			IsKeySend:       invoice.IsKeysend,
			IsAmp:           invoice.IsAmp,
			State:           StringToInvoiceHTLCState(invoice.State),
			AddIndex:        stringToUint64(invoice.AddIndex),
			SettleIndex:     stringToUint64(invoice.SettleIndex),
		})
	}

	return ret, nil
}

// GetPayments API.
func (l *LndRestLightningAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	param := &ListPaymentsRequestOverride{
		MaxPayments: fmt.Sprintf("%d", pagination.BatchSize),
		IndexOffset: fmt.Sprintf("%d", pagination.Offset),
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		param.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		param.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		param.Reversed = true
	}

	if includeIncomplete {
		param.IncludeIncomplete = includeIncomplete
	}

	resp, err := l.HTTPAPI.HTTPListPayments(ctx, l.Request, param)
	if err != nil {
		return nil, err
	}

	ret := &PaymentsResponse{
		Payments: make([]Payment, 0, len(resp.Payments)),
	}

	return ret, nil
}

// SubscribeFailedForwards is used to subscribe to failed forwards.
func (l *LndRestLightningAPI) SubscribeFailedForwards(ctx context.Context, outchan chan RawMessage) error {
	go func() {
		resp, err := l.HTTPAPI.HTTPSubscribeHtlcEvents(ctx, l.Request)
		if err != nil {
			return
		}

		for {
			if ctx.Err() != nil {
				return
			}

			event := <-resp

			if !strings.Contains(strings.ToLower(event.EventType), "forward") {
				continue
			}

			if event.Event.FailureString == "" {
				continue
			}

			m := RawMessage{
				Implementation: l.Name,
				Timestamp:      time.Unix(0, stringToInt64(event.TimestampNs)),
			}

			m.Message, err = json.Marshal(event)
			if err != nil {
				continue
			}

			outchan <- m
		}
	}()

	return nil
}

// GetAPIType API.
func (l *LndRestLightningAPI) GetAPIType() APIType {
	return LndRest
}

// ConnectPeer API.
func (l *LndRestLightningAPI) ConnectPeer(ctx context.Context, id string) error {
	split := strings.Split(id, "@")
	if len(split) != 2 {
		return fmt.Errorf("invalid id")
	}

	addr := &LightningAddressOverride{}
	addr.Pubkey = split[0]
	addr.Host = split[1]

	input := &ConnectPeerRequestOverride{
		Addr:    addr,
		Timeout: "10",
	}
	input.Perm = false

	err := l.HTTPAPI.HTTPPeers(ctx, l.Request, input)
	if err != nil {
		return err
	}

	return nil
}

// GetOnChainAddress API.
func (l *LndRestLightningAPI) GetOnChainAddress(ctx context.Context) (string, error) {
	input := &lnrpc.NewAddressRequest{Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH}

	resp, err := l.HTTPAPI.HTTPNewAddress(ctx, l.Request, input)
	if err != nil {
		return "", err
	}

	return resp.Address, nil
}

// GetOnChainFunds API.
func (l *LndRestLightningAPI) GetOnChainFunds(ctx context.Context) (*Funds, error) {
	resp, err := l.HTTPAPI.HTTPBalance(ctx, l.Request)
	if err != nil {
		return nil, err
	}

	f := &Funds{}
	f.ConfirmedBalance = stringToInt64(resp.ConfirmedBalance)
	f.TotalBalance = stringToInt64(resp.TotalBalance)
	f.LockedBalance = stringToInt64(resp.ReservedBalanceAnchorChan) + stringToInt64(resp.LockedBalance)

	return f, nil
}

// SendToOnChainAddress API.
func (l *LndRestLightningAPI) SendToOnChainAddress(ctx context.Context, address string, sats int64, useUnconfirmed bool, urgency Urgency) (string, error) {
	target := 1
	switch urgency {
	case Urgent:
		target = 1
	case Normal:
		target = 4
	case Low:
		target = 100
	}

	input := &SendCoinsRequestOverride{}
	input.Addr = address
	input.Amount = fmt.Sprintf("%d", sats)
	input.TargetConf = int32(target)
	input.SpendUnconfirmed = useUnconfirmed

	resp, err := l.HTTPAPI.HTTPSendCoins(ctx, l.Request, input)
	if err != nil {
		return "", err
	}

	if len(resp.Txid) == 0 {
		return "", fmt.Errorf("invalid transaction id")
	}

	return resp.Txid, nil
}

// PayInvoice API.
func (l *LndRestLightningAPI) PayInvoice(ctx context.Context, paymentRequest string, sats int64, outgoingChanIds []uint64) (*PaymentResp, error) {
	req := &SendPaymentRequestOverride{}
	req.PaymentRequest = paymentRequest
	// TODO: this is mandatory field but timeout could be configurable
	req.TimeoutSeconds = 20

	satoshis := sats
	if satoshis > 0 {
		req.Amt = fmt.Sprintf("%d", sats)
	} else {
		// Parse amount from invoice in a best effort manner, upon error result is 0...
		satoshis = int64(GetMsatsFromInvoice(paymentRequest) * 1000)
	}

	req.FeeLimitSat = fmt.Sprintf("%d", int64(math.Max(100, math.Round(float64(satoshis)*0.01))))

	req.OutgoingChanIds = make([]string, 0)
	for _, one := range outgoingChanIds {
		req.OutgoingChanIds = append(req.OutgoingChanIds, fmt.Sprintf("%d", one))
	}

	resp, err := l.HTTPAPI.HTTPPayInvoice(ctx, l.Request, req)
	if err != nil {
		return nil, err
	}

	// TODO: fix me
	return &PaymentResp{
		Preimage: resp.PaymentPreimage,
		Hash:     resp.PaymentHash,
		Status:   Pending,
	}, nil
}

// GetPaymentStatus API.
func (l *LndRestLightningAPI) GetPaymentStatus(ctx context.Context, paymentRequest string) (*PaymentResp, error) {
	req := &TrackPaymentRequestOverride{}
	req.NoInflightUpdates = true

	if paymentRequest == "" {
		return nil, fmt.Errorf("missing payment request")
	}

	if strings.HasPrefix(paymentRequest, "ln") {
		paymentHash := GetHashFromInvoice(paymentRequest)
		if paymentHash == "" {
			return nil, fmt.Errorf("bad payment request")
		}

		req.PaymentHash = paymentHash
	} else {
		req.PaymentHash = paymentRequest
	}

	resp, err := l.HTTPAPI.HTTPTrackPayment(ctx, l.Request, req)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(resp.Status) {
	case "succeeded":
		return &PaymentResp{
			Preimage: resp.PaymentPreimage,
			Hash:     req.PaymentHash,
			Status:   Success,
		}, nil
	case "failed":
		return &PaymentResp{
			Preimage: "",
			Hash:     req.PaymentHash,
			Status:   Failed,
		}, nil
	default:
		return &PaymentResp{
			Preimage: "",
			Hash:     req.PaymentHash,
			Status:   Pending,
		}, nil
	}
}

// CreateInvoice API.
func (l *LndRestLightningAPI) CreateInvoice(ctx context.Context, sats int64, preimage string, memo string, expiry time.Duration) (*InvoiceResp, error) {
	req := &InvoiceOverride{}

	req.Memo = memo
	if preimage != "" {
		val, err := hex.DecodeString(preimage)
		if err != nil {
			return nil, err
		}

		req.RPreimage = base64.StdEncoding.EncodeToString(val)
	}

	req.Expiry = fmt.Sprintf("%d", int(expiry.Seconds()))

	if sats > 0 {
		req.Value = fmt.Sprintf("%d", sats)
	}

	resp, err := l.HTTPAPI.HTTPAddInvoice(ctx, l.Request, req)
	if err != nil {
		return nil, err
	}

	if resp.PaymentRequest == "" {
		return nil, fmt.Errorf("no payment request received")
	}

	b, err := base64.StdEncoding.DecodeString(resp.RHash)
	if err != nil {
		return nil, err
	}

	return &InvoiceResp{
		PaymentRequest: resp.PaymentRequest,
		Hash:           hex.EncodeToString(b),
	}, nil
}

// IsInvoicePaid API.
func (l *LndRestLightningAPI) IsInvoicePaid(ctx context.Context, paymentHash string) (bool, error) {
	resp, err := l.HTTPAPI.HTTPLookupInvoice(ctx, l.Request, paymentHash)
	if err != nil {
		return false, err
	}

	return resp.State == "settled", nil
}

func (l *LndRestLightningAPI) convertInitiator(initiator string) CommonInitiator {
	switch initiator {
	case "INITIATOR_LOCAL":
		return Local
	case "INITIATOR_REMOTE":
		return Remote
	default:
		return Unknown
	}
}

// GetChannelCloseInfo API.
func (l *LndRestLightningAPI) GetChannelCloseInfo(ctx context.Context, chanIDs []uint64) ([]CloseInfo, error) {
	var ids []uint64
	resp, err := l.HTTPAPI.HTTPClosedChannels(ctx, l.Request)
	if err != nil {
		return nil, err
	}

	lookup := make(map[uint64]*ChannelCloseSummaryOverride)

	if chanIDs != nil {
		ids = chanIDs
	} else {
		ids = make([]uint64, 0)
	}

	for _, channel := range resp.Channels {
		chanid, err := strconv.ParseUint(channel.ChanId, 10, 64)
		if err != nil {
			continue
		}

		lookup[chanid] = channel
		if chanIDs == nil {
			ids = append(ids, chanid)
		}
	}

	ret := make([]CloseInfo, 0)

	for _, id := range ids {
		if c, ok := lookup[id]; ok {
			typ := UnknownType
			switch c.CloseType {
			case "COOPERATIVE_CLOSE":
				typ = CooperativeType
			case "LOCAL_FORCE_CLOSE":
			case "REMOTE_FORCE_CLOSE":
				typ = ForceType
			}
			ret = append(ret, CloseInfo{
				ChanID:    id,
				Opener:    l.convertInitiator(c.OpenInitiator),
				Closer:    l.convertInitiator(c.CloseInitiator),
				CloseType: typ,
			})
		} else {
			ret = append(ret, UnknownCloseInfo)
		}
	}

	return ret, nil
}

// GetRoute API.
func (l *LndRestLightningAPI) GetRoute(ctx context.Context, source string, destination string, exclusions []Exclusion, msats int64) (DeterminedRoute, error) {
	if !IsValidPubKey(source) || !IsValidPubKey(destination) {
		return nil, ErrPubKeysInvalid
	}

	// exclusions do not work fine with REST
	panic("not implemented")
}
