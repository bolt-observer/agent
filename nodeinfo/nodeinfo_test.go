package nodeinfo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	lightning_api "github.com/bolt-observer/agent/lightningApi"
	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/mitchellh/hashstructure/v2"
)

func getInfoJson(pubkey string) string {
	return fmt.Sprintf(`{
		"identity_pubkey": "%s",
		"alias": "alias",
		"chains": [ {"chain": "bitcoin", "network": "mainnet"}]
	}`, pubkey)
}

func getChannelJson(remote uint64, private, active bool) string {
	return fmt.Sprintf(`{
				"channels": [
				  {
					"chan_id": "1",
					"capacity": "10000",
					"local_balance": "7331",
					"remote_balance": "%d",
					"commit_fee": "2345",
					"commit_weight": "772",
					"fee_per_kw": "1793",
					"unsettled_balance": "0",
					"total_satoshis_sent": "0",
					"total_satoshis_received": "0",
					"num_updates": "1191906",
					"local_chan_reserve_sat": "500",
					"remote_chan_reserve_sat": "500",
					"commitment_type": "ANCHORS",
					"lifetime": "1784230",
					"uptime": "1784230",
					"push_amount_sat": "0",
					"local_constraints": {
					  "chan_reserve_sat": "500",
					  "dust_limit_sat": "354",
					  "max_pending_amt_msat": "49500000",
					  "min_htlc_msat": "1",
					  "csv_delay": 144,
					  "max_accepted_htlcs": 483
					},
					"remote_constraints": {
					  "chan_reserve_sat": "500",
					  "dust_limit_sat": "354",
					  "max_pending_amt_msat": "49500000",
					  "min_htlc_msat": "1",
					  "csv_delay": 144,
					  "max_accepted_htlcs": 483
					},
					"remote_pubkey": "02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c",
					"channel_point": "72003042c278217521ce91dd11ac96ee1ece398c304b514aa3bff9e05329b126:2",
					"csv_delay": 144,
					"private": %s,
					"active": %s,
					"initiator": true,
					"chan_status_flags": "ChanStatusDefault"
				  },
				  {
					"chan_id": "2",
					"capacity": "20000",
					"local_balance": "17325",
					"remote_balance": "0",
					"commit_fee": "2345",
					"commit_weight": "772",
					"fee_per_kw": "1793",
					"unsettled_balance": "0",
					"total_satoshis_sent": "0",
					"total_satoshis_received": "0",
					"num_updates": "213",
					"local_chan_reserve_sat": "354",
					"remote_chan_reserve_sat": "354",
					"commitment_type": "ANCHORS",
					"lifetime": "1784230",
					"uptime": "1784230",
					"push_amount_sat": "0",
					"local_constraints": {
					  "chan_reserve_sat": "354",
					  "dust_limit_sat": "354",
					  "max_pending_amt_msat": "19800000",
					  "min_htlc_msat": "1",
					  "csv_delay": 144,
					  "max_accepted_htlcs": 483
					},
					"remote_constraints": {
					  "chan_reserve_sat": "354",
					  "dust_limit_sat": "354",
					  "max_pending_amt_msat": "19800000",
					  "min_htlc_msat": "1",
					  "csv_delay": 144,
					  "max_accepted_htlcs": 483
					},
					"active": true,
					"remote_pubkey": "02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c",
					"channel_point": "041ba5fed6252813c1913df8a303d59f3d564c53eb3b6f6d218d47fb400f5c31:1",
					"csv_delay": 144,
					"initiator": true,
					"chan_status_flags": "ChanStatusDefault"
				  }
				]
			  }`, remote, strconv.FormatBool(private), strconv.FormatBool(active))
}

func getNodeInfoJson(pubKey string) string {
	return fmt.Sprintf(`
	{"node":{"last_update":1661453114, "pub_key":"%s", "alias":"CrazyConqueror", "addresses":[{"network":"tcp", "addr":"54.147.187.113:9735"}], "color":"#3399ff", "features":{"0":{"name":"data-loss-protect", "is_required":true, "is_known":true}, "5":{"name":"upfront-shutdown-script", "is_required":false, "is_known":true}, "7":{"name":"gossip-queries", "is_required":false, "is_known":true}, "9":{"name":"tlv-onion", "is_required":false, "is_known":true}, "12":{"name":"static-remote-key", "is_required":true, "is_known":true}, "14":{"name":"payment-addr", "is_required":true, "is_known":true}, "17":{"name":"multi-path-payments", "is_required":false, "is_known":true}, "19":{"name":"wumbo-channels", "is_required":false, "is_known":true}, "23":{"name":"anchors-zero-fee-htlc-tx", "is_required":false, "is_known":true}, "31":{"name":"amp", "is_required":false, "is_known":true}, "45":{"name":"explicit-commitment-type", "is_required":false, "is_known":true}, "2023":{"name":"script-enforced-lease", "is_required":false, "is_known":true}}}, "num_channels":3, "total_capacity":"120000", "channels":[{"channel_id":"810130063083110402", "chan_point":"72003042c278217521ce91dd11ac96ee1ece398c304b514aa3bff9e05329b126:2", "last_update":1661455399, "node1_pub":"02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c", "node2_pub":"02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", "capacity":"50000", "node1_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661455399}, "node2_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661395514}}, {"channel_id":"811207584397066241", "chan_point":"041ba5fed6252813c1913df8a303d59f3d564c53eb3b6f6d218d47fb400f5c31:1", "last_update":1661455399, "node1_pub":"02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c", "node2_pub":"02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", "capacity":"20000", "node1_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"19800000", "last_update":1661455399}, "node2_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"19800000", "last_update":1661395514}}, {"channel_id":"821261518687043586", "chan_point":"806d3e1328ddf56958ae3730978c744cac79cfe928b0af8a5a8b52b9f9f66fef:2", "last_update":1661458021, "node1_pub":"02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", "node2_pub":"03005b000a0ed2b172e7608b062bfe2be18df54769a246941b2cebb5ff2658bb83", "capacity":"50000", "node1_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661415314}, "node2_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661458021}}]}
	`, pubKey)
}

func getChanInfo(url string) string {
	s := strings.ReplaceAll(url, "/v1/graph/edge/", "")
	id, err := strconv.Atoi(s)

	if err != nil {
		return ""
	}

	return fmt.Sprintf(`
		{"channel_id":"%d", "chan_point":"72003042c278217521ce91dd11ac96ee1ece398c304b514aa3bff9e05329b126:2", "last_update":1661455399, "node1_pub":"02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c", "node2_pub":"02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256", "capacity":"%d", "node1_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661455399}, "node2_policy":{"time_lock_delta":40, "min_htlc":"1000", "fee_base_msat":"1000", "fee_rate_milli_msat":"1", "disabled":false, "max_htlc_msat":"49500000", "last_update":1661395514}}
	`, id, id*10000)
}

func initTest(t *testing.T) (string, lightning_api.LightingApiCalls, *lightning_api.LndRestLightningApi) {
	pubKey := "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256"
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            pubKey,
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := lightning_api.NewApi(lightning_api.LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
		return "", nil, nil
	}

	d, ok := api.(*lightning_api.LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return "", nil, nil
	}

	return pubKey, api, d
}

func TestSubscription(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	c := NewNodeInfo(ctx, nil)

	if c.IsSubscribed(pubKey, "random_id") {
		t.Fatalf("Should not be subscribed")
		return
	}

	f, _ := filter.NewAllowAllFilter()
	err := c.Subscribe(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		nil,
		f,
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
		return
	}

	if !c.IsSubscribed(pubKey, "random_id") {
		t.Fatalf("Should be subscribed")
		return
	}

	err = c.Unsubscribe(pubKey, "random_id")

	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
		return
	}

	if c.IsSubscribed(pubKey, "random_id") {
		t.Fatalf("Should not be subscribed")
		return
	}
}

func TestBasicFlow(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph/edge") {
			contents = getChanInfo(req.URL.Path)
		} else if strings.Contains(req.URL.Path, "v1/graph/node") {
			contents = getNodeInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewNodeInfo(ctx, nil)

	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	f, _ := filter.NewAllowAllFilter()
	c.Subscribe(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		func(ctx context.Context, report *agent_entities.InfoReport) bool {
			was_called = true
			cancel()
			return true
		},
		f,
	)

	c.EventLoop()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		if !was_called {
			t.Fatalf("Callback was not correctly invoked")
		}
	}
}

func TestBasicFlowFilter(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph/edge") {
			contents = getChanInfo(req.URL.Path)
		} else if strings.Contains(req.URL.Path, "v1/graph/node") {
			contents = getNodeInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewNodeInfo(ctx, nil)

	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	f, _ := filter.NewUnitTestFilter()
	fd := f.(*filter.UnitTestFilter)
	fd.AddAllowChanID(1)
	fd.AddAllowChanID(1337)

	c.Subscribe(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		func(ctx context.Context, report *agent_entities.InfoReport) bool {
			if report.NumChannels == 1 && report.TotalCapacity == 10000 {
				was_called = true
				cancel()
			}

			return true
		},
		f,
	)

	c.EventLoop()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		if !was_called {
			t.Fatalf("Callback was not correctly invoked")
		}
	}
}

func TestContextCanBeNil(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph/edge") {
			contents = getChanInfo(req.URL.Path)
		} else if strings.Contains(req.URL.Path, "v1/graph/node") {
			contents = getNodeInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	c := NewNodeInfo(nil, nil)

	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	f, _ := filter.NewAllowAllFilter()

	c.Subscribe(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		func(ctx context.Context, report *agent_entities.InfoReport) bool {
			was_called = true
			return true
		},
		f,
	)

	go c.EventLoop()

	time.Sleep(2 * time.Second)

	if !was_called {
		t.Fatalf("Callback was not called")
		return
	}
}

func TestGetState(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph/edge") {
			contents = getChanInfo(req.URL.Path)
		} else if strings.Contains(req.URL.Path, "v1/graph/node") {
			contents = getNodeInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	c := NewNodeInfo(ctx, nil)

	f, _ := filter.NewAllowAllFilter()

	resp, err := c.GetState(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		nil,
		f,
	)

	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
		return
	}

	if resp.UniqueId != "random_id" || resp.Node.Alias != "CrazyConqueror" {
		t.Fatalf("GetState returned wrong data: %+v", resp)
		return
	}
}

func TestGetStateCallback(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph/edge") {
			contents = getChanInfo(req.URL.Path)
		} else if strings.Contains(req.URL.Path, "v1/graph/node") {
			contents = getNodeInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	c := NewNodeInfo(ctx, nil)
	var callresp *agent_entities.InfoReport
	callresp = nil

	f, _ := filter.NewAllowAllFilter()

	resp, err := c.GetState(
		pubKey, "random_id", true,
		agent_entities.SECOND,
		func() lightning_api.LightingApiCalls { return api },
		func(ctx context.Context, report *agent_entities.InfoReport) bool {
			callresp = report
			return true
		},
		f,
	)

	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
		return
	}

	if resp.UniqueId != "random_id" || resp.Node.Alias != "CrazyConqueror" {
		t.Fatalf("GetState returned wrong data: %+v", resp)
		return
	}

	if callresp == nil {
		t.Fatalf("GetState returned wrong data")
		return
	}

	hash1, err := hashstructure.Hash(*resp, hashstructure.FormatV2, nil)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
		return
	}

	hash2, err := hashstructure.Hash(*callresp, hashstructure.FormatV2, nil)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
		return
	}

	if hash1 != hash2 {
		t.Fatalf("Two datastructures are not equal")
		return
	}
}
