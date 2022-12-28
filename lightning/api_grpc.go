package lightning

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
	API
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
		Client:      client,
		CleanupFunc: cleanup,
		API:         API{GetNodeInfoFullThreshUseDescribeGraph: 500},
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

// GetForwardingHistory API
func (l *LndGrpcLightningAPI) GetForwardingHistory(ctx context.Context, pagination Pagination) (*ForwardingHistoryResponse, error) {

	req := &lnrpc.ForwardingHistoryRequest{
		NumMaxEvents: uint32(pagination.Num),
		IndexOffset:  uint32(pagination.Offset),
	}

	if pagination.From != nil {
		req.StartTime = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		req.EndTime = uint64(pagination.To.Unix())
	}

	if pagination.Reversed {
		return nil, fmt.Errorf("reverse pagination not supported")
	}

	resp, err := l.Client.ForwardingHistory(ctx, req)

	if err != nil {
		return nil, err
	}

	ret := &ForwardingHistoryResponse{
		ForwardingEvents: make([]ForwardingEvent, 0, len(resp.ForwardingEvents)),
	}

	ret.LastOffsetIndex = uint64(resp.LastOffsetIndex)

	for _, event := range resp.ForwardingEvents {
		ret.ForwardingEvents = append(ret.ForwardingEvents, ForwardingEvent{
			Timestamp:     time.Unix(0, int64(event.TimestampNs)),
			ChanIDIn:      event.ChanIdIn,
			ChanIDOut:     event.ChanIdOut,
			AmountInMsat:  event.AmtInMsat,
			AmountOutMsat: event.AmtOutMsat,
			FeeMsat:       event.FeeMsat,
		})
	}

	return ret, nil
}

// GetInvoices API
func (l *LndGrpcLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	req := &lnrpc.ListInvoiceRequest{
		NumMaxInvoices: pagination.Num,
		IndexOffset:    pagination.Offset,
		PendingOnly:    pendingOnly,
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		req.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		req.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		req.Reversed = true
	}

	resp, err := l.Client.ListInvoices(ctx, req)

	if err != nil {
		return nil, err
	}

	ret := &InvoicesResponse{
		Invoices: make([]Invoice, 0, len(resp.Invoices)),
	}

	ret.LastOffsetIndex = resp.LastIndexOffset
	ret.FirstOffsetIndex = resp.FirstIndexOffset

	for _, invoice := range resp.Invoices {
		ret.Invoices = append(ret.Invoices, Invoice{
			Memo:            invoice.Memo,
			ValueMsat:       invoice.ValueMsat,
			PaidMsat:        invoice.AmtPaidMsat,
			CreationDate:    time.Unix(int64(invoice.CreationDate), 0),
			SettleDate:      time.Unix(int64(invoice.SettleDate), 0),
			PaymentRequest:  invoice.PaymentRequest,
			DescriptionHash: string(invoice.DescriptionHash),
			Expiry:          invoice.Expiry,
			FallbackAddr:    invoice.FallbackAddr,
			CltvExpiry:      invoice.CltvExpiry,
			Private:         invoice.Private,
			IsKeySend:       invoice.IsKeysend,
			IsAmp:           invoice.IsAmp,
			State:           InvoiceHTLCState(invoice.State.Number()),
			AddIndex:        invoice.AddIndex,
			SettleIndex:     invoice.SettleIndex,
		})
	}

	return ret, nil
}

// GetPayments API
func (l *LndGrpcLightningAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	req := &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: includeIncomplete,
		MaxPayments:       pagination.Num,
		IndexOffset:       pagination.Offset,
	}

	/* TODO: Need to upgrade to 0.15.5!
	if pagination.From != nil {
		req.CreationDateStart = uint64(pagination.From.Unix())
	}

	if pagination.To != nil {
		req.CreationDateEnd = uint64(pagination.To.Unix())
	}
	*/
	if pagination.From != nil || pagination.To != nil {
		return nil, fmt.Errorf("from and to are not yet supported")
	}

	if pagination.Reversed {
		req.Reversed = true
	}

	resp, err := l.Client.ListPayments(ctx, req)

	if err != nil {
		return nil, err
	}

	ret := &PaymentsResponse{
		Payments: make([]Payment, 0, len(resp.Payments)),
	}

	ret.LastOffsetIndex = resp.LastIndexOffset
	ret.FirstOffsetIndex = resp.FirstIndexOffset

	for _, payment := range resp.Payments {

		pay := Payment{
			PaymentHash:     payment.PaymentHash,
			ValueMsat:       payment.ValueMsat,
			FeeMsat:         payment.FeeMsat,
			PaymentPreimage: payment.PaymentPreimage,
			PaymentRequest:  payment.PaymentRequest,
			PaymentStatus:   PaymentStatus(payment.Status.Number()),
			CreationTime:    time.Unix(0, payment.CreationTimeNs),
			Index:           payment.PaymentIndex,
			FailureReason:   PaymentFailureReason(payment.FailureReason.Number()),
			HTLCAttempts:    make([]HTLCAttempt, 0),
		}

		for _, htlc := range payment.Htlcs {

			//for _, hops := range htlc.Route.Hops

			attempt := HTLCAttempt{
				ID:      htlc.AttemptId,
				Status:  HTLCStatus(htlc.Status.Number()),
				Attempt: time.Unix(0, htlc.AttemptTimeNs),
				Resolve: time.Unix(0, htlc.AttemptTimeNs),
			}

			pay.HTLCAttempts = append(pay.HTLCAttempts, attempt)
		}

		ret.Payments = append(ret.Payments, pay)
	}

	return ret, nil
}
