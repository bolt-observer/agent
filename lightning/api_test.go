package lightning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/lightningnetwork/lnd/lnrpc"
)

func TestApiSelection(t *testing.T) {

	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	// ApiType in NewApi() is a preference that can be overriden through data

	// Invalid API type
	api := NewAPI(APIType(500), func() (*entities.Data, error) {
		return &data, nil
	})

	if api != nil {
		t.Fatalf("API should be nil")
	}

	// Use CLN
	name, closer := ClnSocketServer(t, nil)
	defer closer()

	temp := data.Endpoint
	data.Endpoint = name
	api = NewAPI(ClnSocket, func() (*entities.Data, error) {
		return &data, nil
	})
	data.Endpoint = temp

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok := api.(*LndClnSocketLightningAPI)
	if !ok {
		t.Fatalf("Should be CLN_SOCKET")
	}

	// Use gRPC
	api = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	// Use REST
	api = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	v := int(LndGrpc)
	data.ApiType = &v

	api = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	v = int(LndRest)
	data.ApiType = &v

	api = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	// Invalid type
	v = 500
	data.ApiType = &v

	api = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

}

func TestGrpcDoesNotOpenConnection(t *testing.T) {
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}

func TestRestDoesNotOpenConnection(t *testing.T) {
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}

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

func TestNodeInfoFull(t *testing.T) {
	mock := &MockLightningAPI{}
	resp, err := getNodeInfoFullTemplate(context.Background(), mock, 100, true, true)
	if err != nil {
		t.Fatalf("Error getting node info: %v", err)
		return
	}
	if resp.Node.PubKey != "fake" || resp.NumChannels != 2 || resp.TotalCapacity != 3 {
		t.Fatalf("Wrong data returned")
		return
	}

	if resp.Channels[0].Private || !resp.Channels[1].Private {
		t.Fatalf("Wrong private data returned")
		return
	}

	if strings.Contains(mock.Trace, "wrong") || strings.Contains(mock.Trace, "false") || strings.Contains(mock.Trace, "describegraph") {
		t.Fatalf("Invoked wrong method")
		return
	}

	if !strings.Contains(mock.Trace, "getchaninfo") {
		t.Fatalf("GetChanInfo was not invoked")
		return
	}

	clone := &NodeInfoAPIExtended{
		NodeInfoAPI: resp.NodeInfoAPI,
		Channels:    make([]NodeChannelAPIExtended, 0),
	}

	if clone.NumChannels != 2 || clone.TotalCapacity != 3 {
		t.Fatalf("Wrong data returned from clone")
	}
}

func TestNodeInfoFullPublic(t *testing.T) {
	mock := &MockLightningAPI{}
	resp, err := getNodeInfoFullTemplate(context.Background(), mock, 100, true, false)
	if err != nil {
		t.Fatalf("Error getting node info: %v", err)
		return
	}
	if resp.Node.PubKey != "fake" || resp.NumChannels != 1 || resp.TotalCapacity != 1 {
		t.Fatalf("Wrong data returned")
		return
	}

	if int(resp.NumChannels) != len(resp.Channels) {
		t.Fatalf("Not all channels reported")
		return
	}

}

func TestNodeInfoFullWithDescribeGraph(t *testing.T) {
	mock := &MockLightningAPI{}
	resp, err := getNodeInfoFullTemplate(context.Background(), mock, 1, true, true)
	if err != nil {
		t.Fatalf("Error getting node info: %v", err)
		return
	}
	if resp.Node.PubKey != "fake" || resp.NumChannels != 2 || resp.TotalCapacity != 3 {
		t.Fatalf("Wrong data returned")
		return
	}

	if strings.Contains(mock.Trace, "wrong") || strings.Contains(mock.Trace, "false") {
		t.Fatalf("Invoked wrong method")
		return
	}

	if !strings.Contains(mock.Trace, "describegraph") {
		t.Fatalf("DescribeGraph was not invoked")
		return
	}

	if strings.Contains(mock.Trace, "getchaninfo") {
		t.Fatalf("GetChanInfo was invoked")
		return
	}

	if resp.Node.PubKey != "fake" || resp.NumChannels != 2 || resp.TotalCapacity != 3 {
		t.Fatalf("Wrong data returned")
		return
	}

	if resp.Channels[0].Private || !resp.Channels[1].Private {
		t.Fatalf("Wrong private data returned")
		return
	}

	clone := &NodeInfoAPIExtended{
		NodeInfoAPI: resp.NodeInfoAPI,
		Channels:    make([]NodeChannelAPIExtended, 0),
	}

	if clone.NumChannels != 2 || clone.TotalCapacity != 3 {
		t.Fatalf("Wrong data returned from clone")
	}
}

func TestRawMessageSerialization(t *testing.T) {
	var (
		err  error
		data entities.Data
	)
	const FixtureSecret = "fixture-grpc.secret"

	if _, err := os.Stat(FixtureSecret); errors.Is(err, os.ErrNotExist) {
		// If file with credentials does not exist succeed
		return
	}

	content, err := ioutil.ReadFile(FixtureSecret)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return
	}

	client, _, cleanup, err := GetClient(func() (*entities.Data, error) {
		return &data, nil
	})
	if err != nil {
		t.Fatalf("GetClient failed: %v\n", err)
	}

	defer cleanup()

	resp, err := client.ListPayments(context.Background(), &lnrpc.ListPaymentsRequest{MaxPayments: 10, IncludeIncomplete: true})
	if err != nil {
		t.Fatalf("ListPayments failed: %v\n", err)
	}

	for _, one := range resp.Payments {
		raw := RawMessage{}

		raw.Timestamp = time.Unix(0, one.CreationTimeNs)
		raw.Implementation = "lnd"
		raw.Message, err = json.Marshal(one)
		if err != nil {
			t.Fatalf("Message marshal error: %v\n", err)
		}

		msg, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("Wrapped message marshal error: %v\n", err)
		}

		fmt.Printf("JSON |%s|\n", msg)
	}

	//t.Fail()

}
