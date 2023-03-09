package boltz

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	filter "github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
	bip39 "github.com/tyler-smith/go-bip39"
)

const LnRegTestPathPrefix = "/tmp/lnregtest-data/dev_network/"

func getLocalLnd(t *testing.T, name string, endpoint string) agent_entities.NewAPICall {
	data := &common_entities.Data{}
	x := int(api.LndGrpc)
	data.Endpoint = endpoint
	data.ApiType = &x

	content, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/tls.cert", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
	data.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/data/chain/bitcoin/regtest/admin.macaroon", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
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

func mine(numBlocks int) error {
	_, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s/bitcoin", LnRegTestPathPrefix), "-generate", fmt.Sprintf("%d", numBlocks)).Output()

	return err
}

func TestSwap(t *testing.T) {
	const (
		Node     = "D"
		BoltzUrl = "http://localhost:9001"
	)

	// Useful commands:
	// $ curl -X POST localhost:9001/swapstatus -d '{ "id": "mmsUtR" }'  -H "Content-Type: application/json"
	// {"status":"invoice.set"}
	// $ bitcoin-cli -datadir=/tmp/lnregtest-data/dev_network/bitcoin -generate 1

	nodes := map[string]string{
		"A": "localhost:11009",
		"C": "localhost:11011",
		"D": "localhost:11012",
		"E": "localhost:11013",
		"F": "localhost:11014",
		"G": "localhost:11015",
	}

	ln := getLocalLnd(t, Node, nodes[Node])
	if ln == nil {
		fmt.Printf("Ignoring swap test since regtest network is not available\n")
		return
	}
	// Sanity check of node
	ctx := context.Background()
	lnAPI, err := ln()
	assert.NotNil(t, lnAPI)
	assert.NoError(t, err)
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
	_, err = p.BoltzAPI.GetNodes()
	assert.NoError(t, err)

	// Override entropy (or id needs to be random)
	entropy, err := bip39.NewEntropy(SecretBitSize)
	assert.NoError(t, err)
	p.CryptoAPI.MasterSecret = entropy

	err = p.Execute(1339, []byte(`{ "target": "InboundLiquidityNodePercent", "percentage": 90} `), msgCallback)
	assert.NoError(t, err)

	for i := 0; i < 5; i++ {
		mine(1)
		time.Sleep(10 * time.Second)
	}

	time.Sleep(5 * time.Minute)
	t.Fail()
}
