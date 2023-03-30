package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const FixtureDir = "./fixtures"

func TestObtainData(t *testing.T) {
	// This is used as a way to gather raw fixtures

	var data entities.Data
	const FixtureSecret = "fixture.secret"

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

	api, err := NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
		return
	}

	d, ok := api.(*LndRestLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return
	}

	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		client := &http.Client{Transport: d.Transport}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		bodyData, _ := io.ReadAll(resp.Body)
		name := strings.ReplaceAll(strings.ReplaceAll(req.URL.Path, "/", "_"), "_v1_", "")

		if name == "channels" {
			var channels *Channels

			err = json.Unmarshal(bodyData, &channels)
			if err != nil {
				t.Fatalf("Failed to unmarshal")
				return nil, err
			}

			if len(channels.Channels) > 0 {
				// Play with some numbers
				channels.Channels[0].ChanID = "1337"
				channels.Channels[0].RemoteBalance = "1337"
				channels.Channels[0].LocalBalance = "7331"
				channels.Channels[0].Active = false
				channels.Channels[0].Private = true
			}

			bodyData, err = json.Marshal(channels)
			if err != nil {
				t.Fatalf("Failed to marshal")
				return nil, err
			}
		} else if name == "graph" {
			var graph *Graph
			err = json.Unmarshal(bodyData, &graph)
			if err != nil {
				t.Fatalf("Failed to unmarshal")
				return nil, err
			}

			graph.GraphNodeOverride = graph.GraphNodeOverride[0:5]
			graph.GraphEdgesOverride = graph.GraphEdgesOverride[0:5]

			bodyData, err = json.Marshal(graph)
			if err != nil {
				t.Fatalf("Failed to marshal")
				return nil, err
			}
		} else if strings.Contains(name, "graph_node") {
			name = "graph_node"
		} else if strings.Contains(name, "graph_edge") {
			name = "graph_edge"
		}

		f, err := os.OpenFile(fmt.Sprintf("%s/%s.json", FixtureDir, name), os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			t.Fatalf("Could not open file: %v", err)
			return nil, err
		}
		err = f.Truncate(0)
		if err != nil {
			t.Fatalf("Could not truncate file: %v", err)
			return nil, err
		}
		defer f.Close()

		fmt.Fprintf(f, "%s\n", string(bodyData))

		resp.Body = io.NopCloser(bytes.NewBuffer(bodyData))
		return resp, nil
	}

	resp, err := api.GetChannelCloseInfo(context.Background(), nil)
	assert.NoError(t, err)
	fmt.Printf("%+v\n", resp)
}

func common(t *testing.T, name string) ([]byte, *LndRestLightningAPI, LightingAPICalls) {
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
	f, err := os.OpenFile(fmt.Sprintf("%s/%s.json", FixtureDir, name), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
		return nil, nil, nil
	}
	defer f.Close()

	contents, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
		return nil, nil, nil
	}

	api, err := NewAPI(LndRest, func() (*entities.Data, error) {
		return &data, nil
	})
	assert.NoError(t, err)

	if api == nil {
		t.Fatalf("API should not be nil")
		return nil, nil, nil
	}

	d, ok := api.(*LndRestLightningAPI)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return nil, nil, nil
	}

	return contents, d, api
}

func TestGetInfo(t *testing.T) {
	contents, d, api := common(t, "getinfo")
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/getinfo") {
			t.Fatalf("URL should contain v1/getinfo")
		}

		r := io.NopCloser(bytes.NewReader(contents))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	result, err := api.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
		return
	}

	if result.IdentityPubkey != pubKey || result.Alias != "CrazyConqueror" || result.Chain != "bitcoin" || result.Network != "mainnet" {
		t.Fatalf("GetInfo got wrong response: %v", result)
		return
	}

	if !strings.HasPrefix(result.Version, "lnd-") || !result.IsSyncedToChain || !result.IsSyncedToGraph {
		t.Fatalf("GetInfo got wrong response: %v", result)
		return
	}
}

func TestGetChannels(t *testing.T) {
	contents, d, api := common(t, "channels")

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/channels") {
			t.Fatalf("URL should contain v1/channels")
		}

		r := io.NopCloser(bytes.NewReader(contents))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	result, err := api.GetChannels(context.Background())
	if err != nil {
		t.Fatalf("GetChannels failed: %v", err)
		return
	}

	for _, c := range result.Channels {
		if !utils.ValidatePubkey(c.RemotePubkey) {
			t.Fatalf("Invalid pubkey: %v", c.RemotePubkey)
			return
		}

		if c.ChanID == 1337 {
			if c.LocalBalance != 1337 && c.RemoteBalance != 3771 && c.Active != false && c.Private {
				t.Fatalf("Wrong response")
				return
			}
		}
	}
}

func TestDescribeGraph(t *testing.T) {
	contents, d, api := common(t, "graph")

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/graph") {
			t.Fatalf("URL should contain v1/graph")
		}

		r := io.NopCloser(bytes.NewReader(contents))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	_, err := api.DescribeGraph(context.Background(), false)
	if err != nil {
		t.Fatalf("DescribeGraph failed: %v", err)
		return
	}
}

func TestGetNodeInfo(t *testing.T) {
	contents, d, api := common(t, "graph_node")
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/graph/node") {
			t.Fatalf("URL should contain v1/graph/node")
		}

		r := io.NopCloser(bytes.NewReader(contents))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	result, err := api.GetNodeInfo(context.Background(), pubKey, true)
	if err != nil {
		t.Fatalf("GetNodeInfo failed: %v", err)
		return
	}

	if result.Node.PubKey != pubKey || result.Node.Alias != "CrazyConqueror" {
		t.Fatalf("GetNodeInfo got wrong response: %v", result)
		return
	}

	if len(result.Channels) != int(result.NumChannels) {
		t.Fatalf("GetNodeInfo got wrong channel response: %v", result)
		return
	}
}

func TestGetChanInfo(t *testing.T) {
	contents, d, api := common(t, "graph_edge")
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"
	chanid := uint64(810130063083110402)

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/graph/edge") {
			t.Fatalf("URL should contain v1/graph/edge")
		}

		r := io.NopCloser(bytes.NewReader(contents))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	result, err := api.GetChanInfo(context.Background(), chanid)
	if err != nil {
		t.Fatalf("GetChanInfo failed: %v", err)
		return
	}

	if result.ChannelID != chanid || result.ChanPoint != "72003042c278217521ce91dd11ac96ee1ece398c304b514aa3bff9e05329b126:2" || (result.Node1Pub != pubKey && result.Node2Pub != pubKey) {
		t.Fatalf("GetChanInfo got wrong response: %v", result)
	}
}

func TestConnectPeer(t *testing.T) {
	_, d, api := common(t, "getinfo")
	pubkey := "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a"
	host := "161.97.184.185:9735"

	r := io.NopCloser(bytes.NewReader([]byte{}))
	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/peers") {
			t.Fatalf("URL should contain v1/peers")
		}
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	err := api.ConnectPeer(context.Background(), fmt.Sprintf("%s@%s", pubkey, host))
	assert.NoError(t, err)
	err = api.ConnectPeer(context.Background(), fmt.Sprintf("%s%s", pubkey, host))
	assert.Error(t, err)
}

func TestGetOnChainAddress(t *testing.T) {
	data, d, api := common(t, "newaddress")

	r := io.NopCloser(bytes.NewReader(data))
	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/newaddress") {
			t.Fatalf("URL should contain v1/newaddress")
		}
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	resp, err := api.GetOnChainAddress(context.Background())
	assert.NoError(t, err)

	assert.NotEqual(t, 0, len(resp))
}

func TestGetChannelCloseInfo(t *testing.T) {
	data, d, api := common(t, "close")

	// Mock
	d.HTTPAPI.DoFunc = func(req *http.Request) (*http.Response, error) {
		r := io.NopCloser(bytes.NewReader(data))
		if !strings.Contains(req.URL.Path, "v1/channels/closed") {
			t.Fatalf("URL should contain v1/channels/closed")
		}
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	resp, err := api.GetChannelCloseInfo(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp))
	assert.Equal(t, CooperativeType, resp[0].CloseType)
	assert.Equal(t, Remote, resp[0].Opener)
	assert.Equal(t, Local, resp[0].Closer)

	id := resp[0].ChanID
	resp, err = api.GetChannelCloseInfo(context.Background(), []uint64{id, 1337})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp))
	assert.Equal(t, CooperativeType, resp[0].CloseType)
	assert.Equal(t, Remote, resp[0].Opener)
	assert.Equal(t, Local, resp[0].Closer)
	assert.Equal(t, id, resp[0].ChanID)

	assert.Equal(t, UnknownType, resp[1].CloseType)
	assert.Equal(t, Unknown, resp[1].Opener)
	assert.Equal(t, Unknown, resp[1].Closer)
	assert.Equal(t, uint64(0), resp[1].ChanID)
}
