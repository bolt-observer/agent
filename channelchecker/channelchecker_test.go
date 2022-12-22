package channelchecker

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	lightning_api "github.com/bolt-observer/agent/lightning_api"
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

func getBrokenChannels() string {
	return `channels: [ { "chan_id": {`
}

func getChannelJson(remote uint64, private, active bool) string {
	return fmt.Sprintf(`{
				"channels": [
				  {
					"chan_id": "1",
					"capacity": "50000",
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

func TestRemoveQueryParams(t *testing.T) {
	if removeQueryParams("rediss://burek:6379?ssl_cert_reqs=none") !=
		"rediss://burek:6379" {
		t.Fatalf("Query string still present")
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
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 2 && report.UniqueId == "random_id" {
				was_called = true
			}

			cancel()
			return true
		},
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

func TestBasicFlowFilterOne(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	f, _ := filter.NewUnitTestFilter()
	fd := f.(*filter.UnitTestFilter)
	fd.AddAllowChanId(1)
	fd.AddAllowChanId(1337)

	c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
			Filter:               f,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 1 && report.UniqueId == "random_id" {
				was_called = true
			}

			cancel()
			return true
		},
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

func TestBasicFlowFilterTwo(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	f, _ := filter.NewUnitTestFilter()
	fd := f.(*filter.UnitTestFilter)
	fd.AddAllowPubKey("02004c625d622245606a1ea2c1c69cfb4516b703b47945a3647713c05fe4aaeb1c")

	c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
			Filter:               f,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 2 && report.UniqueId == "random_id" {
				was_called = true
			}

			cancel()
			return true
		},
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
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	c := NewDefaultChannelChecker(nil, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 2 && report.UniqueId == "random_id" {
				was_called = true
			}

			return true
		},
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
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)

	resp, err := c.GetState(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		nil,
	)

	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
		return
	}

	if len(resp.ChangedChannels) != 2 || resp.UniqueId != "random_id" {
		t.Fatalf("GetState returned bad data: %+v", resp)
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
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)

	var callresp *agent_entities.ChannelBalanceReport
	callresp = nil

	resp, err := c.GetState(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			callresp = report
			return true
		},
	)

	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
		return
	}

	if len(resp.ChangedChannels) != 2 || resp.UniqueId != "random_id" {
		t.Fatalf("GetState returned bad data: %+v", resp)
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

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)

	if c.IsSubscribed(pubKey, "random_id") {
		t.Fatalf("Should not be subscribed")
		return
	}

	err := c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
		return
	}

	// Second subscribe works without errors
	err = c.Subscribe(
		pubKey, "random_id",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		nil,
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

func TestPrivateChannelsExcluded(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, true, true)
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: false,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 1 {
				was_called = true
			}

			cancel()
			return true
		},
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

func TestInactiveFlow(t *testing.T) {
	pubKey, api, d := initTest(t)

	step := 0

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			if step == 0 {
				contents = getChannelJson(1337, false, true)
			} else if step == 1 {
				contents = getChannelJson(1337, false, false)
			} else if step == 2 {
				contents = getChannelJson(1337, false, true)
			}
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			switch step {
			case 0:
				if len(report.ChangedChannels) == 2 {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 0")
				}
			case 1:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].Active == false && report.ChangedChannels[0].ActivePrevious == true {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 1")
				}
			case 2:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].Active == true && report.ChangedChannels[0].ActivePrevious == false {
					step += 1
					cancel()
				} else {
					cancel()
					t.Fatalf("Bad step 2")
				}
			}

			return true

		},
	)

	c.EventLoop()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		if step < 3 {
			t.Fatalf("Callback was not correctly invoked")
		}
	}
}

func TestChange(t *testing.T) {
	pubKey, api, d := initTest(t)

	step := 0

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			if step == 0 {
				contents = getChannelJson(1337, false, true)
			} else if step == 1 {
				contents = getChannelJson(1339, false, true)
			} else if step == 2 {
				contents = getChannelJson(1337, false, true)
			}
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			switch step {
			case 0:
				if len(report.ChangedChannels) == 2 {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 0")
				}
			case 1:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].RemoteNominator == 1339 && report.ChangedChannels[0].RemoteNominatorDiff == 2 {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 1")
				}
			case 2:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].RemoteNominator == 1337 && report.ChangedChannels[0].RemoteNominatorDiff == -2 {
					step += 1
					cancel()
				} else {
					cancel()
					t.Fatalf("Bad step 2")
				}
			}

			return true

		},
	)

	c.EventLoop()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		if step < 3 {
			t.Fatalf("Callback was not correctly invoked")
		}
	}
}

func TestPubkeyWrong(t *testing.T) {
	pubKey, api, d := initTest(t)

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("wrong")
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	c := NewDefaultChannelChecker(context.Background(), time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	err := c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		nil,
	)

	if err == nil {
		t.Fatalf("Wrong pubkey was not detected")
		return
	}
}

func TestKeepAliveIsSent(t *testing.T) {
	pubKey, api, d := initTest(t)

	step := 0
	success := false

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			if step == 0 {
				contents = getChannelJson(1337, false, true)
			} else if step >= 1 {
				contents = getChannelJson(1339, false, true)
			}

			step += 1
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
			NoopInterval:         2 * time.Second,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			//fmt.Fprintf(os.Stderr, "%d) %+v\n\n", step, report.ChangedChannels)

			if step == 1 {
				if len(report.ChangedChannels) != 2 {
					t.Fatalf("Not correct change step %d", step)
					cancel()
				}
			} else if step == 2 {
				if len(report.ChangedChannels) != 1 {
					t.Fatalf("Not correct change step %d", step)
					cancel()
				}
			} else if step > 2 {
				if len(report.ChangedChannels) != 0 {
					t.Fatalf("Not correct change step %d", step)
					cancel()
				}

				success = true

				cancel()
			}

			return true

		},
	)

	c.EventLoop()

	select {
	case <-time.After(6 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		// nothing
	}

	if !success {
		t.Fatalf("Did not go through all step - keepalive not sent?")
	}
}

func TestKeepAliveIsNotSentWhenError(t *testing.T) {
	pubKey, api, d := initTest(t)

	step := 0
	success := true

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			if step == 0 {
				contents = getChannelJson(1337, false, true)
			} else if step >= 1 {
				contents = getBrokenChannels()
			}

			step += 1
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
			NoopInterval:         2 * time.Second,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			//fmt.Fprintf(os.Stderr, "%d) %+v\n\n", step, report.ChangedChannels)

			if step == 1 {
				if len(report.ChangedChannels) != 2 {
					t.Fatalf("Not correct change step %d", step)
					cancel()
				}
			} else if step > 2 {
				success = false

				cancel()
			}

			return true

		},
	)

	c.EventLoop()

	select {
	case <-time.After(6 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		// nothing
	}

	if !success {
		t.Fatalf("Did not go through all step - keepalive was sent?")
	}
}

func TestChangeIsCachedWhenCallbackFails(t *testing.T) {
	pubKey, api, d := initTest(t)

	step := 0

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			if step == 0 {
				contents = getChannelJson(1337, false, true)
			} else if step == 1 {
				contents = getChannelJson(1338, false, true)
			} else if step == 2 {
				contents = getChannelJson(1339, false, true)
			}
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			switch step {
			case 0:
				if len(report.ChangedChannels) == 2 {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 0")
				}
			case 1:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].RemoteNominator == 1338 && report.ChangedChannels[0].RemoteNominatorDiff == 1 {
					step += 1
				} else {
					cancel()
					t.Fatalf("Bad step 1")
				}
				// Fail on purpose
				return false
			case 2:
				if len(report.ChangedChannels) == 1 && report.ChangedChannels[0].RemoteNominator == 1339 && report.ChangedChannels[0].RemoteNominatorDiff == 2 {
					step += 1
					cancel()
				} else {
					cancel()
					t.Fatalf("Bad step 2")
				}
			}

			return true

		},
	)

	c.EventLoop()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		if step < 3 {
			t.Fatalf("Callback was not correctly invoked")
		}
	}
}

func TestGraphIsRequested(t *testing.T) {
	// Note that this is still not reflected in reported diff at the moment (we just assert that we are able to peridocially call DescribeGraph)

	pubKey, api, d := initTest(t)

	success := false

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		} else if strings.Contains(req.URL.Path, "v1/graph") {
			contents = `
			{"nodes":[{"last_update":1659296984,"pub_key":"020003b9499a97c8dfbbab6b196319db37ba9c37bccb60477f3c867175f417988e","alias":"BJCR_BTCPayServer","addresses":[{"network":"tcp","addr":"95.217.192.209:9735"}],"color":"#3399ff","features":{"0":{"name":"data-loss-protect","is_required":true,"is_known":true},"12":{"name":"static-remote-key","is_required":true,"is_known":true},"14":{"name":"payment-addr","is_required":true,"is_known":true},"17":{"name":"multi-path-payments","is_known":true},"2023":{"name":"script-enforced-lease","is_known":true},"23":{"name":"anchors-zero-fee-htlc-tx","is_known":true},"31":{"name":"amp","is_known":true},"45":{"name":"explicit-commitment-type","is_known":true},"5":{"name":"upfront-shutdown-script","is_known":true},"7":{"name":"gossip-queries","is_known":true},"9":{"name":"tlv-onion","is_known":true}}},{"last_update":1657199384,"pub_key":"0200072fd301cb4a680f26d87c28b705ccd6a1d5b00f1b5efd7fe5f998f1bbb1f1","alias":"OutaSpace 🚀","addresses":[{"network":"tcp","addr":"176.28.11.68:9760"},{"network":"tcp","addr":"nzslu33ecbokyn32teza2peiiiuye43ftom7jvnuhsxdbg3vhw7w3aqd.onion:9760"}],"color":"#123456","features":{"1":{"name":"data-loss-protect","is_known":true},"11":{"name":"unknown"},"13":{"name":"static-remote-key","is_known":true},"14":{"name":"payment-addr","is_required":true,"is_known":true},"17":{"name":"multi-path-payments","is_known":true},"27":{"name":"unknown"},"5":{"name":"upfront-shutdown-script","is_known":true},"55":{"name":"unknown"},"7":{"name":"gossip-queries","is_known":true},"8":{"name":"tlv-onion","is_required":true,"is_known":true}}},{"last_update":1618162974,"pub_key":"0200081eaa41b5661d3b512f5aae9d6abfb11ba1497a354e9217d9a18fbaa1e76b","alias":"0200081eaa41b5661d3b","addresses":[{"network":"tcp","addr":"lm63zodngkzqbol6lgadijh5p5xm6ltbekfxlbofvmnbkvi5cnzrzdid.onion:9735"}],"color":"#3399ff","features":{"0":{"name":"data-loss-protect","is_required":true,"is_known":true},"12":{"name":"static-remote-key","is_required":true,"is_known":true},"14":{"name":"payment-addr","is_required":true,"is_known":true},"17":{"name":"multi-path-payments","is_known":true},"5":{"name":"upfront-shutdown-script","is_known":true},"7":{"name":"gossip-queries","is_known":true},"9":{"name":"tlv-onion","is_known":true}}},{"last_update":1660845145,"pub_key":"020016201d389a44840f1f33be29288952f67c8ef6b3f98726fda180b4185ca6e2","alias":"AlasPoorYorick","addresses":[{"network":"tcp","addr":"7vuykfnmgkarlk4xjew4ea6lj7qwbbggbox4b72abupu7sn24geajzyd.onion:9735"}],"color":"#604bee","features":{"0":{"name":"data-loss-protect","is_required":true,"is_known":true},"12":{"name":"static-remote-key","is_required":true,"is_known":true},"14":{"name":"payment-addr","is_required":true,"is_known":true},"17":{"name":"multi-path-payments","is_known":true},"2023":{"name":"script-enforced-lease","is_known":true},"23":{"name":"anchors-zero-fee-htlc-tx","is_known":true},"31":{"name":"amp","is_known":true},"45":{"name":"explicit-commitment-type","is_known":true},"5":{"name":"upfront-shutdown-script","is_known":true},"7":{"name":"gossip-queries","is_known":true},"9":{"name":"tlv-onion","is_known":true}}},{"last_update":1660753871,"pub_key":"02001828ca7eb8e44d4d78b5c1ea609cd3744be823c22cd69d895eff2f9345892d","alias":"nodl-lnd-s010-042","addresses":[{"network":"tcp","addr":"185.150.160.210:4042"}],"color":"#000000","features":{"0":{"name":"data-loss-protect","is_required":true,"is_known":true},"12":{"name":"static-remote-key","is_required":true,"is_known":true},"14":{"name":"payment-addr","is_required":true,"is_known":true},"17":{"name":"multi-path-payments","is_known":true},"2023":{"name":"script-enforced-lease","is_known":true},"23":{"name":"anchors-zero-fee-htlc-tx","is_known":true},"31":{"name":"amp","is_known":true},"45":{"name":"explicit-commitment-type","is_known":true},"5":{"name":"upfront-shutdown-script","is_known":true},"7":{"name":"gossip-queries","is_known":true},"9":{"name":"tlv-onion","is_known":true}}}],"edges":[{"channel_id":"553951550347608065","capacity":"37200","chan_point":"ede04f9cfc1bb5373fd07d8af9c9b8b5a85cfe5e323b7796eb0a4d0dce5d5058:1","node1_pub":"03bd3466efd4a7306b539e2314e69efc6b1eaee29734fcedd78cf81b1dde9fedf8","node2_pub":"03c3d14714b78f03fd6ea4997c2b540a4139258249ea1d625c03b68bb82f85d0ea"},{"channel_id":"554317687705305088","capacity":"1000000","chan_point":"cfd0ae79fc150c2c3c4068ceca74bc26652bb2691624379aba9e28b197a78d6a:0","node1_pub":"02eccebd9ed98f6d267080a58194dbe554a2b33d976eb95bb7c116d00fd64c4a13","node2_pub":"02ee4469f2b686d5d02422917ac199602ce4c366a7bfaac1099e3ade377677064d"},{"channel_id":"554460624201252865","capacity":"1000000","chan_point":"c0a8d3428f562c232d86be399eb4497934e7e0390fa79e6860bcb65e7b0dd4fe:1","node1_pub":"02eccebd9ed98f6d267080a58194dbe554a2b33d976eb95bb7c116d00fd64c4a13","node2_pub":"02ee4469f2b686d5d02422917ac199602ce4c366a7bfaac1099e3ade377677064d"},{"channel_id":"554494709160148993","capacity":"200000","chan_point":"06bbac25ed610feb1d07316d1be8b8ba6850ee1dd96cc1d5439159bfe992be5a:1","node1_pub":"03bd3466efd4a7306b539e2314e69efc6b1eaee29734fcedd78cf81b1dde9fedf8","node2_pub":"03cbf298b068300be33f06c947b9d3f00a0f0e8089da3233f5db37e81d3a596fe1"},{"channel_id":"554495808645955584","capacity":"2000000","chan_point":"2392c45431c064269e4eaeccb0476ac32e56485d84e104064636aea896d1e439:0","node1_pub":"022e74ed3ddd3f590fd6492e60b20dcad7303f17e1ffd882fb33bb3f6c88f64398","node2_pub":"02ee4469f2b686d5d02422917ac199602ce4c366a7bfaac1099e3ade377677064d"}]}
			`
			success = true
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	c := NewDefaultChannelChecker(ctx, time.Duration(0), true, true, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
			GraphPollInterval:    1 * time.Second,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			return true
		},
	)

	c.EventLoop()

	select {
	case <-time.After(4 * time.Second):
		t.Fatal("Took too long")
	case <-ctx.Done():
		// nothing
	}

	if !success {
		t.Fatalf("DescribeGraph was not called")
	}
}

func TestBasicFlowRedis(t *testing.T) {
	pubKey, api, d := initTest(t)

	mr := miniredis.RunT(t)
	mr.Addr()

	err := os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s/0", mr.Addr()))
	if err != nil {
		t.Fatalf("Could not set REDIS_URL")
		return
	}

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents = getInfoJson("02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256")
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents = getChannelJson(1337, false, true)
		}

		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))

	// Use redis
	c := NewChannelChecker(ctx, NewRedisChannelCache(), time.Duration(0), true, false, nil)
	// Make everything a bit faster
	c.OverrideLoopInterval(1 * time.Second)
	was_called := false

	c.Subscribe(
		pubKey, "",
		func() lightning_api.LightingApiCalls { return api },
		agent_entities.ReportingSettings{
			AllowedEntropy:       64,
			PollInterval:         agent_entities.SECOND,
			AllowPrivateChannels: true,
		},
		func(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
			if len(report.ChangedChannels) == 2 {
				was_called = true
			}

			cancel()
			return true
		},
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
