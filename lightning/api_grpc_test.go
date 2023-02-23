package lightning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	gomock "github.com/golang/mock/gomock"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/assert"

	mocks "github.com/bolt-observer/agent/lightning/mocks"
)

func TestObtainDataGrpc(t *testing.T) {
	// This is used as a way to gather raw fixtures

	var data entities.Data
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

	if _, err := os.Stat(FixtureDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(FixtureDir, os.ModePerm)
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

	api := NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return
	}

	_, ok := api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
		return
	}

	//api.GetInfo(context.Background())
	//api.GetChannels(context.Background())
	//api.DescribeGraph(context.Background(), false)
	//api.GetNodeInfo(context.Background(), "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", true)
	//api.GetChanInfo(context.Background(), uint64(810130063083110402))
	//api.GetForwardingHistory(context.Background(), Pagination{})

	/*
		ret, err := api.GetInvoices(context.Background(), false, Pagination{BatchSize: 500})
		if err != nil {
			t.Fatalf("Error %v", err)
		}

		for _, v := range ret.Invoices {
			fmt.Printf("%v\n", v)
		}
	*/

	//api.ConnectPeer(context.Background(), "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a@161.97.184.185:9735")
	//api.GetOnChainAddress(context.Background())
	//resp, err := api.GetOnChainFunds(context.Background())
	resp, err := api.IsInvoicePaid(context.Background(), "6c0a4f30b9bf9b6d1ca3f15ed7782ea2d52c67017cf0bd4f31c185724c37bccb")
	assert.NoError(t, err)
	fmt.Printf("%+v\n", resp)

	t.Fail()
}

func commonGrpc(t *testing.T, name string, m *mocks.MockLightningClient, mr *mocks.MockRouterClient) ([]byte, LightingAPICalls) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	ignore := 4
	data := entities.Data{
		PubKey:               pubKey,
		MacaroonHex:          dummyMac,
		Endpoint:             "bolt.observer:443",
		CertVerificationType: &ignore,
	}

	// Prepare mock data
	f, err := os.OpenFile(fmt.Sprintf("%s/%s_grpc.json", FixtureDir, name), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
		return nil, nil
	}
	defer f.Close()

	contents, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
		return nil, nil
	}

	api := NewAPI(LndGrpc, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return nil, nil
	}

	d, ok := api.(*LndGrpcLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
		return nil, nil
	}

	d.Client = m
	d.RouterClient = mr

	return contents, api
}

func TestGetInfoGrpc(t *testing.T) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "getinfo", m, nil)

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

	if !strings.HasPrefix(resp.Version, "lnd-") || !resp.IsSyncedToChain || !resp.IsSyncedToGraph {
		t.Fatalf("GetInfo got wrong response: %v", resp)
		return
	}

	//t.Fail()
}

func TestGetChannelsGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "channels", m, nil)

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

		if c.ChanID == 1337 {
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
	data, api := commonGrpc(t, "graph", m, nil)

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
	data, api := commonGrpc(t, "graph_node", m, nil)

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
	data, api := commonGrpc(t, "graph_edge", m, nil)

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

	if resp.ChannelID != chanid || resp.ChanPoint != "72003042c278217521ce91dd11ac96ee1ece398c304b514aa3bff9e05329b126:2" || (resp.Node1Pub != pubKey && resp.Node2Pub != pubKey) {
		t.Fatalf("GetChanInfo got wrong response: %v", resp)
	}
}

func TestConnectPeerGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "getinfo", m, nil)

	var info *lnrpc.GetInfoResponse
	err := json.Unmarshal(data, &info)
	assert.NoError(t, err)

	pubkey := "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a"
	host := "161.97.184.185:9735"

	m.
		EXPECT().
		ConnectPeer(gomock.Any(), gomock.Eq(&lnrpc.ConnectPeerRequest{
			Addr:    &lnrpc.LightningAddress{Host: host, Pubkey: pubkey},
			Perm:    false,
			Timeout: 10})).
		Return(&lnrpc.ConnectPeerResponse{}, nil)

	err = api.ConnectPeer(context.Background(), fmt.Sprintf("%s@%s", pubkey, host))
	assert.NoError(t, err)

	m.
		EXPECT().
		ConnectPeer(gomock.Any(), gomock.Eq(&lnrpc.ConnectPeerRequest{
			Addr:    &lnrpc.LightningAddress{Host: host, Pubkey: pubkey},
			Perm:    false,
			Timeout: 10})).
		Return(&lnrpc.ConnectPeerResponse{}, fmt.Errorf("already connected to peer"))

	err = api.ConnectPeer(context.Background(), fmt.Sprintf("%s@%s", pubkey, host))
	assert.NoError(t, err)

	m.
		EXPECT().
		ConnectPeer(gomock.Any(), gomock.Eq(&lnrpc.ConnectPeerRequest{
			Addr:    &lnrpc.LightningAddress{Host: host, Pubkey: pubkey},
			Perm:    false,
			Timeout: 10})).
		Return(&lnrpc.ConnectPeerResponse{}, fmt.Errorf("other error"))

	err = api.ConnectPeer(context.Background(), fmt.Sprintf("%s@%s", pubkey, host))
	assert.Error(t, err)
}

func TestGetOnChainAddressGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "newaddress", m, nil)

	var info *lnrpc.NewAddressResponse
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		NewAddress(gomock.Any(), gomock.Any()).
		Return(info, nil)

	resp, err := api.GetOnChainAddress(context.Background())
	assert.NoError(t, err)

	assert.NotEqual(t, 0, len(resp))
}

func TestGetOnChainFundsGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "balance", m, nil)

	var info *lnrpc.WalletBalanceResponse
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		WalletBalance(gomock.Any(), gomock.Any()).
		Return(info, nil)

	resp, err := api.GetOnChainFunds(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(89476363), resp.TotalBalance)
	assert.Equal(t, int64(89476363), resp.ConfirmedBalance)
}

func TestSendToOnChainAddressGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	data, api := commonGrpc(t, "sendcoins", m, nil)

	var info *lnrpc.SendCoinsResponse
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}
	addr := "bcrt1q852e4etsvdg9nsets630dr06m5lvswz2ad7shq"

	target := &lnrpc.SendCoinsRequest{
		Addr:             addr,
		Amount:           10000,
		TargetConf:       int32(1),
		SpendUnconfirmed: false,
	}

	m.
		EXPECT().
		SendCoins(gomock.Any(), gomock.Eq(target)).
		Return(info, nil)

	resp, err := api.SendToOnChainAddress(context.Background(), addr, 10000, false, Urgent)
	assert.NoError(t, err)
	assert.Equal(t, "dafd01e2ed745676160087f5db6ba36addd576d42d3ee608375ff5d9bca4ca19", resp)
}

type Pair[T, U any] struct {
	First  T
	Second U
}

type PayInvoiceRespFunc func(resp *PaymentResp, err error)

func TestPayInvoiceGrpc(t *testing.T) {
	for _, one := range []Pair[string, PayInvoiceRespFunc]{
		{First: "payinvoice_noroute", Second: func(resp *PaymentResp, err error) {
			assert.Error(t, err)
		}},
		{First: "payinvoice_inflight", Second: func(resp *PaymentResp, err error) {
			assert.NoError(t, err)
			assert.Equal(t, Pending, resp.Status)
		}},
		{First: "payinvoice_ok", Second: func(resp *PaymentResp, err error) {
			assert.NoError(t, err)
			assert.Equal(t, Success, resp.Status)
			assert.Equal(t, "629b553a37c1c274633019dc772466250080ac2b419670a3537099eef06995ca", resp.Preimage)
		}},
	} {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockLightningClient(ctrl)
		mr := mocks.NewMockRouterClient(ctrl)
		data, api := commonGrpc(t, one.First, m, mr)

		var payment *lnrpc.Payment

		err := json.Unmarshal(data, &payment)
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
			return
		}

		c := mocks.NewMockRouter_SendPaymentV2Client(ctrl)
		c.EXPECT().
			Recv().
			Return(payment, nil)

		mr.
			EXPECT().
			SendPaymentV2(gomock.Any(), gomock.Any()).
			Return(c, nil)

		resp, err := api.PayInvoice(context.Background(), "lnbcrt13370n1p3lwhhnpp5zfvpdpwp77wmgpyawatm550uv9cvx0hv9hgukxd6u0pqqdhcdymsdqqcqzpgxqyz5vqsp5xsum9s4cpkvw27f6smk4daxqkpjgmah8xxhs8ty34fm4srmwfvjs9qyyssqlx5zk7j8lfeelzmpsk0mmwp3583tl52j8us2q9nt05vrmtp3sasxzc8wyjchrum67sllzr52gjz26rcrye6y4vrlpr6pyv9jhhlrp2cq52rxf5", 0, []uint64{128642860515328})
		ctrl.Finish()

		one.Second(resp, err)
	}
}

func TestGetPaymentStatusGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	mr := mocks.NewMockRouterClient(ctrl)
	data, api := commonGrpc(t, "track_payment", m, mr)

	var payment *lnrpc.Payment

	err := json.Unmarshal(data, &payment)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	c := mocks.NewMockRouter_TrackPaymentV2Client(ctrl)
	c.EXPECT().
		Recv().
		Return(payment, nil)

	mr.
		EXPECT().
		TrackPaymentV2(gomock.Any(), gomock.Any()).
		Return(c, nil)

	resp, err := api.GetPaymentStatus(context.Background(), "12581685c1f79db4049d7757ba51fc6170c33eec2dd1cb19bae3c20036f86937")
	assert.NoError(t, err)
	assert.Equal(t, Success, resp.Status)
}

func TestCreateInvoiceGrpc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockLightningClient(ctrl)
	mr := mocks.NewMockRouterClient(ctrl)
	data, api := commonGrpc(t, "addinvoice", m, mr)

	var info *lnrpc.AddInvoiceResponse

	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("Failed to unmarshal info: %v", err)
		return
	}

	m.
		EXPECT().
		AddInvoice(gomock.Any(), gomock.Any()).
		Return(info, nil)

	resp, err := api.CreateInvoice(context.Background(), 1337, "", "", 5*time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, "6c0a4f30b9bf9b6d1ca3f15ed7782ea2d52c67017cf0bd4f31c185724c37bccb", resp.Hash)
}

type IsInvoicePaidFunc func(resp bool, err error)

func TestIsInvoicePaidGrpc(t *testing.T) {
	for _, one := range []Pair[string, IsInvoicePaidFunc]{
		{First: "invoice_notpaid", Second: func(resp bool, err error) {
			assert.NoError(t, err)
			assert.Equal(t, false, resp)
		}},
		{First: "invoice_paid", Second: func(resp bool, err error) {
			assert.NoError(t, err)
			assert.Equal(t, true, resp)
		}},
	} {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockLightningClient(ctrl)
		data, api := commonGrpc(t, one.First, m, nil)

		var invoice *lnrpc.Invoice

		err := json.Unmarshal(data, &invoice)
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
			return
		}

		m.
			EXPECT().
			LookupInvoice(gomock.Any(), gomock.Any()).
			Return(invoice, nil)

		resp, err := api.IsInvoicePaid(context.Background(), "6c0a4f30b9bf9b6d1ca3f15ed7782ea2d52c67017cf0bd4f31c185724c37bccb")

		ctrl.Finish()

		one.Second(resp, err)
	}
}
