package lightning_api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	gomock "github.com/golang/mock/gomock"
	"github.com/lightningnetwork/lnd/lnrpc"

	mocks "github.com/bolt-observer/agent/lightning_api/mocks"
)

func TestObtainDataGrpc(t *testing.T) {
	// This is used as a way to gather raw fixtures

	var data entities.Data
	const FIXTURE_SECRET = "fixture-grpc.secret"

	if _, err := os.Stat(FIXTURE_SECRET); errors.Is(err, os.ErrNotExist) {
		// If file with credentials does not exist succeed
		return
	}

	content, err := ioutil.ReadFile(FIXTURE_SECRET)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return
	}

	if _, err := os.Stat(FIXTURE_DIR); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(FIXTURE_DIR, os.ModePerm)
		if err != nil {
			t.Fatalf("Could not create directory: %v", err)
			return
		}
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return
	}

	api := NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return
	}

	_, ok := api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
		return
	}

	//api.GetInfo(context.Background())
	//api.GetChannels(context.Background())
	//api.DescribeGraph(context.Background(), false)
	//api.GetNodeInfo(context.Background(), "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", true)
	//api.GetChanInfo(context.Background(), uint64(810130063083110402))
}

func commonGrpc(t *testing.T, name string, m *mocks.MockLightningClient) ([]byte, LightingApiCalls) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            pubKey,
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	// Prepare mock data
	f, err := os.OpenFile(fmt.Sprintf("%s/%s_grpc.json", FIXTURE_DIR, name), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
		return nil, nil
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
		return nil, nil
	}

	api := NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return nil, nil
	}

	d, ok := api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
		return nil, nil
	}

	d.Client = m

	return contents, api
}

func TestGetInfoGrpc(t *testing.T) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "getinfo", m)

	var info *lnrpc.GetInfoResponse
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		GetInfo(gomock.Any(), gomock.Any()).
		Return(info, nil)

	resp, err := api.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("Failed to call GetInfo %v", err)
		return
	}
	if resp.IdentityPubkey != pubKey || resp.Alias != "CrazyConqueror" || resp.Chain != "bitcoin" || resp.Network != "mainnet" {
		t.Fatalf("GetInfo got wrong response: %v", resp)
		return
	}
}

func TestGetChannelsGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "channels", m)

	var channels *lnrpc.ListChannelsResponse
	err := json.Unmarshal(data, &channels)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		ListChannels(gomock.Any(), gomock.Any()).
		Return(channels, nil)

	resp, err := api.GetChannels(context.Background())
	if err != nil {
		t.Fatalf("Failed to call GetChannels %v", err)
		return
	}

	for _, c := range resp.Channels {
		if !utils.ValidatePubkey(c.RemotePubkey) {
			t.Fatalf("Invalid pubkey: %v", c.RemotePubkey)
			return
		}

		if c.ChanId == 1337 {
			if c.LocalBalance != 1337 {
				t.Fatalf("Wrong response")
				return
			}
		}
	}
}

func TestDescribeGraphGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "graph", m)

	var graph *lnrpc.ChannelGraph
	err := json.Unmarshal(data, &graph)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		DescribeGraph(gomock.Any(), gomock.Any()).
		Return(graph, nil)

	resp, err := api.DescribeGraph(context.Background(), false)
	if err != nil {
		t.Fatalf("Failed to call DescribeGraph %v", err)
		return
	}

	if resp == nil {
		t.Fatalf("Bad response")
	}
}

func TestGetNodeInfoGrpc(t *testing.T) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "graph_node", m)

	var info *lnrpc.NodeInfo
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		GetNodeInfo(gomock.Any(), gomock.Eq(&lnrpc.NodeInfoRequest{PubKey: pubKey, IncludeChannels: true})).
		Return(info, nil)

	resp, err := api.GetNodeInfo(context.Background(), pubKey, true)
	if err != nil {
		t.Fatalf("Failed to call DescribeGraph %v", err)
		return
	}

	if resp == nil {
		t.Fatalf("Bad response")
	}

	if resp.Node.PubKey != pubKey || resp.Node.Alias != "CrazyConqueror" {
		t.Fatalf("GetNodeInfo got wrong response: %v", resp)
		return
	}

	if len(resp.Channels) != int(resp.NumChannels) {
		t.Fatalf("GetNodeInfo got wrong channel response: %v", resp)
		return
	}
}

func TestGetChanInfoGrpc(t *testing.T) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"
	chanid := uint64(810130063083110402)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "graph_edge", m)

	var channel *lnrpc.ChannelEdge
	err := json.Unmarshal(data, &channel)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		GetChanInfo(gomock.Any(), gomock.Eq(&lnrpc.ChanInfoRequest{ChanId: chanid})).
		Return(channel, nil)

	resp, err := api.GetChanInfo(context.Background(), chanid)
	if err != nil {
		t.Fatalf("Failed to call GetChanInfo %v", err)
		return
	}

	if resp == nil {
		t.Fatalf("Bad response")
	}

	if resp.ChannelId != chanid || (resp.Node1Pub != pubKey && resp.Node2Pub != pubKey) {
		t.Fatalf("GetChanInfo got wrong response: %v", resp)
	}
}
