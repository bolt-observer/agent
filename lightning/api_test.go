package lightning

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/assert"
)

func TestApiSelection(t *testing.T) {
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	ignore := 4
	data := entities.Data{
		PubKey:               "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:          dummyMac,
		Endpoint:             "bolt.observer:443",
		CertVerificationType: &ignore,
	}

	// ApiType in NewApi() is a preference that can be overriden through data

	// Invalid API type
	api, err := NewAPI(APIType(500), func() (*entities.Data, error) {
		return &data, nil
	})
	assert.Error(t, err)
	if api != nil {
		t.Fatalf("API should be nil")
	}

	// Use CLN
	name, closer := ClnSocketServer(t, nil)
	defer closer()

	temp := data.Endpoint
	data.Endpoint = name
	api, err = NewAPI(ClnSocket, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)
	data.Endpoint = temp

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok := api.(*ClnSocketLightningAPI)
	if !ok {
		t.Fatalf("Should be CLN_SOCKET")
	}

	// Use gRPC
	api, err = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	// Use REST
	api, err = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	v := int(LndGrpc)
	data.ApiType = &v

	api, err = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	v = int(LndRest)
	data.ApiType = &v

	api, err = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

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

	api, err = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

}

func TestGrpcDoesNotOpenConnection(t *testing.T) {
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"
	ignore := 4
	data := entities.Data{
		PubKey:               "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:          dummyMac,
		Endpoint:             "bolt.observer:443",
		CertVerificationType: &ignore,
	}

	api, err := NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api, err = NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}

func TestRestDoesNotOpenConnection(t *testing.T) {
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"
	ignore := 4
	data := entities.Data{
		PubKey:               "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:          dummyMac,
		Endpoint:             "bolt.observer:443",
		CertVerificationType: &ignore,
	}

	api, err := NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api, err = NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
	}
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

	content, err := os.ReadFile(FixtureSecret)
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

		_, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("Wrapped message marshal error: %v\n", err)
		}
	}

	//t.Fail()

}

func TestCtxError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(8*time.Second))

	done := false

	go func() {
		for i := 0; i < 100; i++ {
			if ctx.Err() != nil {
				//fmt.Printf("Error: %v\n", ctx.Err())
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		done = true
	}()

	cancel()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, true, done)

	//t.Fail()
}
