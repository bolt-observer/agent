package lightning

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type MockLightningAPI struct {
	Trace string
}

func (m *MockLightningAPI) Cleanup() {}
func (m *MockLightningAPI) GetInfo(ctx context.Context) (*InfoAPI, error) {
	m.Trace += "getinfo"
	return &InfoAPI{IdentityPubkey: "fake"}, nil
}

func (m *MockLightningAPI) GetChannels(ctx context.Context) (*ChannelsAPI, error) {
	m.Trace += "getchannels"
	return &ChannelsAPI{Channels: []ChannelAPI{{ChanID: 1, Capacity: 1}, {ChanID: 2, Capacity: 2, Private: true}}}, nil
}

func (m *MockLightningAPI) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphAPI, error) {
	m.Trace += "describegraph" + strconv.FormatBool(unannounced)

	return &DescribeGraphAPI{Nodes: []DescribeGraphNodeAPI{{PubKey: "fake"}},
		Channels: []NodeChannelAPI{{ChannelID: 1, Capacity: 1, Node1Pub: "fake"}, {ChannelID: 2, Capacity: 2, Node2Pub: "fake"}}}, nil
}
func (m *MockLightningAPI) GetNodeInfoFull(ctx context.Context, channels, unannounced bool) (*NodeInfoAPIExtended, error) {
	// Do not use
	m.Trace += "wrong"
	return nil, nil
}
func (m *MockLightningAPI) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoAPI, error) {
	m.Trace += "getnodeinfo" + pubKey + strconv.FormatBool(channels)

	return &NodeInfoAPI{Node: DescribeGraphNodeAPI{PubKey: "fake"},
		Channels:    []NodeChannelAPI{{ChannelID: 1, Capacity: 1}},
		NumChannels: 2, TotalCapacity: 3}, nil
}

func (m *MockLightningAPI) GetChanInfo(ctx context.Context, chanID uint64) (*NodeChannelAPI, error) {
	m.Trace += fmt.Sprintf("getchaninfo%d", chanID)
	return &NodeChannelAPI{ChannelID: chanID, Capacity: chanID}, nil
}

func (m *MockLightningAPI) GetInvoices(ctx context.Context, pendingOnly bool, pagination Pagination) (*InvoicesResponse, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) SubscribeForwards(ctx context.Context, since time.Time, batchSize uint16, maxErrors uint16) (<-chan []ForwardingEvent, <-chan ErrorData) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetPayments(ctx context.Context, includeIncomplete bool, pagination Pagination) (*PaymentsResponse, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetInvoicesRaw(ctx context.Context, pendingOnly bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetPaymentsRaw(ctx context.Context, includeIncomplete bool, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetForwardsRaw(ctx context.Context, pagination RawPagination) ([]RawMessage, *ResponseRawPagination, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetAPIType() APIType {
	panic("not implemented")
}

func (m *MockLightningAPI) ConnectPeer(ctx context.Context, id string) error {
	panic("not implemented")
}

func (m *MockLightningAPI) GetOnChainAddress(ctx context.Context) (string, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetOnChainFunds(ctx context.Context) (*Funds, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) SendToOnChainAddress(ctx context.Context, address string, sats int64, useUnconfirmed bool, urgency Urgency) (string, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) PayInvoice(ctx context.Context, paymentRequest string, sats int64, outgoingChanIds []uint64) (*PaymentResp, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetPaymentStatus(ctx context.Context, paymentHash string) (*PaymentResp, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) CreateInvoice(ctx context.Context, sats int64, preimage string, memo string, expiry time.Duration) (*InvoiceResp, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) IsInvoicePaid(ctx context.Context, paymentHash string) (bool, error) {
	panic("not implemented")
}

func (m *MockLightningAPI) GetChannelCloseInfo(ctx context.Context, chanIDs []uint64) ([]CloseInfo, error) {
	panic("not implemented")
}
