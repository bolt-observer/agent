package channelchecker

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	lightning_api "github.com/bolt-observer/agent/lightning_api"
	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
)

func TestBasicFlow(t *testing.T) {

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
		return
	}

	d, ok := api.(*lightning_api.LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return
	}

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents =
				`{
				"identity_pubkey": "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256",
				"alias": "alias",
				"chains": [ {"chain": "bitcoin", "network": "mainnet"}]
			}`
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents =
				`
			{
				"channels": [
				  {
					"chan_id": "1",
					"capacity": "50000",
					"local_balance": "7331",
					"remote_balance": "1337",
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
					"private": true,
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
			  }`
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
		pubKey,
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

func TestPrivateChannelsExcluded(t *testing.T) {

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
		return
	}

	d, ok := api.(*lightning_api.LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
		return
	}

	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		contents := ""
		if strings.Contains(req.URL.Path, "v1/getinfo") {
			contents =
				`{
				"identity_pubkey": "02b67e55fb850d7f7d77eb71038362bc0ed0abd5b7ee72cc4f90b16786c69b9256",
				"alias": "alias",
				"chains": [ {"chain": "bitcoin", "network": "mainnet"}]
			}`
		} else if strings.Contains(req.URL.Path, "v1/channels") {
			contents =
				`
			{
				"channels": [
				  {
					"chan_id": "1",
					"capacity": "50000",
					"local_balance": "7331",
					"remote_balance": "1337",
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
					"private": true,
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
			  }`
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
		pubKey,
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
