package boltz

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	filter "github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
	bip39 "github.com/tyler-smith/go-bip39"
)

func getLocalLnd(t *testing.T, name string, endpoint string) agent_entities.NewAPICall {
	const Prefix = "/tmp/lnregtest-data/dev_network/lnnodes"

	data := &common_entities.Data{}
	x := int(api.LndGrpc)
	data.Endpoint = endpoint
	data.ApiType = &x

	content, err := os.ReadFile(fmt.Sprintf("%s/%s/tls.cert", Prefix, name))
	assert.NoError(t, err)
	data.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := os.ReadFile(fmt.Sprintf("%s/%s/data/chain/bitcoin/regtest/admin.macaroon", Prefix, name))
	assert.NoError(t, err)
	data.MacaroonHex = hex.EncodeToString(macBytes)

	return func() (api.LightingAPICalls, error) {
		api, err := api.NewAPI(api.LndGrpc, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func msgCallback(msg agent_entities.PluginMessage) error {
	fmt.Printf("%+v\n", msg)
	return nil
}

func TestSwap(t *testing.T) {
	const (
		Node     = "E"
		BoltzUrl = "http://localhost:9001"
	)

	nodes := map[string]string{
		"A": "localhost:11009",
		"C": "localhost:11011",
		"D": "localhost:11012",
		"E": "localhost:11013",
		"F": "localhost:11014",
		"G": "localhost:11015",
	}

	ln := getLocalLnd(t, Node, nodes[Node])
	// Sanity check of node
	ctx := context.Background()
	lnAPI, err := ln()
	if lnAPI == nil || err != nil {
		// in order to prevent failures on CI, you need e2e env started locally
		return
	}

	info, err := lnAPI.GetInfo(ctx)
	assert.NoError(t, err)
	assert.Equal(t, Node, info.Alias)

	funds, err := lnAPI.GetOnChainFunds(ctx)
	assert.NoError(t, err)
	assert.Greater(t, funds.ConfirmedBalance, int64(1_000_000))

	// Construct plugin
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	p, err := NewPlugin(ln, f, getMockCliCtx(BoltzUrl))
	assert.NoError(t, err)
	r, err := p.BoltzAPI.GetNodes()
	assert.NoError(t, err)
	fmt.Printf("Nodes: %+v\n", r)

	// Override entropy
	entropy, err := bip39.NewEntropy(SecretBitSize)
	assert.NoError(t, err)
	p.CryptoAPI.MasterSecret = entropy

	err = p.Execute(1338, []byte(`{ "target": "OutboundLiquidityNodePercent", "percentage": 12.3} `), msgCallback)
	assert.NoError(t, err)

	time.Sleep(5 * time.Minute)
	t.Fail()
}
