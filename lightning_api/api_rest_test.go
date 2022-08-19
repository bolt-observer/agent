package lightning_api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
)

const FIXTURE_DIR = "./fixtures"

func TestObtainData(t *testing.T) {
	// This is used as a way to gather raw fixtures

	var data entities.Data
	const FIXTURE_SECRET = "fixture.secret"

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

	api := NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return
	}

	d, ok := api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return
	}

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		client := &http.Client{Transport: d.Transport}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		bodyData, _ := ioutil.ReadAll(resp.Body)
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
				channels.Channels[0].ChanId = "1337"
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
		}

		f, err := os.OpenFile(fmt.Sprintf("%s/%s.json", FIXTURE_DIR, name), os.O_WRONLY|os.O_CREATE, 0644)
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

		resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyData))
		return resp, nil
	}

	// All methods

	_, err = api.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("fail %v", err)
		return
	}
	_, err = api.GetChannels(context.Background())
	if err != nil {
		t.Fatalf("fail %v", err)
		return
	}
	_, err = api.DescribeGraph(context.Background(), false)
	if err != nil {
		t.Fatalf("fail %v", err)
		return
	}
}

func common(t *testing.T, name string) ([]byte, *LndRestLightningApi, LightingApiCalls) {
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
	f, err := os.OpenFile(fmt.Sprintf("%s/%s.json", FIXTURE_DIR, name), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
		return nil, nil, nil
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
		return nil, nil, nil
	}

	api := NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return nil, nil, nil
	}

	d, ok := api.(*LndRestLightningApi)
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
	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/getinfo") {
			t.Fatalf("URL should contain v1/getinfo")
		}

		r := ioutil.NopCloser(bytes.NewReader(contents))

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
}

func TestGetChannels(t *testing.T) {
	contents, d, api := common(t, "channels")

	// Mock
	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/channels") {
			t.Fatalf("URL should contain v1/channels")
		}

		r := ioutil.NopCloser(bytes.NewReader(contents))

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

		if c.ChanId == 1337 {
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
	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/graph") {
			t.Fatalf("URL should contain v1/graph")
		}

		r := ioutil.NopCloser(bytes.NewReader(contents))

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
