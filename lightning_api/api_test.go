package lightning_api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
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
	api := NewApi(ApiType(3), func() (*entities.Data, error) {
		return &data, nil
	})

	if api != nil {
		t.Fatalf("API should be nil")
	}

	// Use gRPC
	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok := api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	// Use REST
	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	v := int(LND_GRPC)
	data.ApiType = &v

	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	v = int(LND_REST)
	data.ApiType = &v

	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	// Invalid type
	v = 3
	data.ApiType = &v

	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningApi)
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

	api := NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
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

	api := NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}

type MockLightningApi struct {
	Trace string
}

func (m *MockLightningApi) Cleanup() {}
func (m *MockLightningApi) GetInfo(ctx context.Context) (*InfoApi, error) {
	m.Trace += "getinfo"
	return &InfoApi{IdentityPubkey: "fake"}, nil
}

func (m *MockLightningApi) GetChannels(ctx context.Context) (*ChannelsApi, error) {
	m.Trace += "getchannels"
	return &ChannelsApi{Channels: []ChannelApi{{ChanId: 1, Capacity: 1}, {ChanId: 2, Capacity: 2, Private: true}}}, nil
}

func (m *MockLightningApi) DescribeGraph(ctx context.Context, unannounced bool) (*DescribeGraphApi, error) {
	m.Trace += "describegraph" + strconv.FormatBool(unannounced)

	return &DescribeGraphApi{Nodes: []DescribeGraphNodeApi{{PubKey: "fake"}},
		Channels: []NodeChannelApi{{ChannelId: 1, Capacity: 1, Node1Pub: "fake"}, {ChannelId: 2, Capacity: 2, Node2Pub: "fake"}}}, nil
}
func (m *MockLightningApi) GetNodeInfoFull(ctx context.Context, channels, unannounced bool) (*NodeInfoApiExtended, error) {
	// Do not use
	m.Trace += "wrong"
	return nil, nil
}
func (m *MockLightningApi) GetNodeInfo(ctx context.Context, pubKey string, channels bool) (*NodeInfoApi, error) {
	m.Trace += "getnodeinfo" + pubKey + strconv.FormatBool(channels)

	return &NodeInfoApi{Node: DescribeGraphNodeApi{PubKey: "fake"},
		Channels:    []NodeChannelApi{{ChannelId: 1, Capacity: 1}, {ChannelId: 2, Capacity: 2}},
		NumChannels: 2, TotalCapacity: 3}, nil
}

func (m *MockLightningApi) GetChanInfo(ctx context.Context, chanId uint64) (*NodeChannelApi, error) {
	m.Trace += fmt.Sprintf("getchaninfo%d", chanId)
	return &NodeChannelApi{ChannelId: chanId, Capacity: chanId}, nil
}

func TestNodeInfoFull(t *testing.T) {
	mock := &MockLightningApi{}
	resp, err := getNodeInfoFull(mock, 100, context.Background(), true, true)
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
}

func TestNodeInfoFullWithDescribeGraph(t *testing.T) {
	mock := &MockLightningApi{}
	resp, err := getNodeInfoFull(mock, 1, context.Background(), true, true)
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
}
