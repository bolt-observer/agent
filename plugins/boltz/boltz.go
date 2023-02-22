// //go:build plugins

package boltz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	boltz "github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	plugins "github.com/bolt-observer/agent/plugins"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
)

const (
	Symbol   = "BTC"
	BoltzUrl = "https://boltz.exchange/api"
)

// BoltzPlugin struct.
type BoltzPlugin struct {
	BoltzAPI    *boltz.Boltz
	ChainParams *chaincfg.Params
	plugins.Plugin
}

type SwapStatus int

const (
	Pending SwapStatus = iota
	Successful
	Error
	ServerError
	Refunded
	Abandoned
)

func NewBoltzPlugin(lightning entities.NewAPICall, network string) *BoltzPlugin {
	params := chaincfg.MainNetParams
	switch network {
	case "mainnet":
		params = chaincfg.MainNetParams
	case "testnet":
		params = chaincfg.TestNet3Params
	case "regtest":
		params = chaincfg.RegressionNetParams
	case "simnet":
		params = chaincfg.SimNetParams
	}

	resp := &BoltzPlugin{
		ChainParams: &params,
		BoltzAPI: &boltz.Boltz{
			URL: BoltzUrl,
		},
	}
	if lightning == nil {
		return nil
	}

	resp.Lightning = lightning
	return resp
}

func (b *BoltzPlugin) EnsureConnected(ctx context.Context) error {
	nodes, err := b.BoltzAPI.GetNodes()
	if err != nil {
		return err
	}

	lapi := b.Lightning()
	if lapi == nil {
		return fmt.Errorf("could not get lightning api")
	}
	defer lapi.Cleanup()

	node, hasNode := nodes.Nodes[Symbol]

	if !hasNode {
		return fmt.Errorf("could not find Boltz LND node for symbol %s", Symbol)
	}

	if len(node.URIs) == 0 {
		return fmt.Errorf("could not find URIs for Boltz LND node for symbol %s", Symbol)
	}

	success := false
	last := ""

	for _, url := range node.URIs {
		err = lapi.ConnectPeer(ctx, url)
		if err == nil {
			success = true
			break
		} else {
			last = err.Error()
		}
	}

	if success {
		return nil
	}

	return fmt.Errorf("could not connect to Boltz LND node - %s", last)
}

func newKeys() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privateKey, err := btcec.NewPrivateKey()

	if err != nil {
		return nil, nil, err
	}

	publicKey := privateKey.PubKey()

	return privateKey, publicKey, err
}

func newPreimage() ([]byte, []byte, error) {
	preimage := make([]byte, 32)
	_, err := rand.Read(preimage)

	if err != nil {
		return nil, nil, err
	}

	preimageHash := sha256.Sum256(preimage)

	return preimage, preimageHash[:], nil
}

func (b *BoltzPlugin) Check(id string) {

	resp, err := b.BoltzAPI.SwapStatus(id)
	if err != nil {
		fmt.Printf("SwapStatus error: %v\n", err)
		return
	}

	fmt.Printf("%+v\n", resp)

	stopListening := make(chan bool, 1)
	status := make(chan *boltz.SwapStatusResponse, 1)

	go b.BoltzAPI.StreamSwapStatus(id, status, stopListening)

outer:
	for {
		fmt.Printf("!\n")
		select {
		case s := <-status:
			fmt.Printf("Status %s\n", s.Status)
		case <-time.After(1 * time.Second):
			fmt.Printf("Timed out waiting for status\n")
			break outer
		}

		fmt.Printf(".\n")
	}

	stopListening <- true
}

func (b *BoltzPlugin) Swap() {
	const BlockEps = 10

	_, preimageHash, err := newPreimage()
	if err != nil {
		fmt.Printf("Error creating preimage %v\n", err)
		return
	}

	priv, pub, err := newKeys()
	if err != nil {
		fmt.Printf("Error creating keys %v\n", err)
		return
	}

	response, err := b.BoltzAPI.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(preimageHash),
		RefundPublicKey: hex.EncodeToString(pub.SerializeCompressed()),
	})

	if err != nil {
		fmt.Printf("Error creating swap %v\n", err)
		return
	}

	fmt.Printf("Response: %+v\n", response)

	redeemScript, err := hex.DecodeString(response.RedeemScript)
	if err != nil {
		fmt.Printf("Error decoding redeem script %v\n", err)
		return
	}

	fmt.Printf("Timeout %v\n", response.TimeoutBlockHeight)

	err = boltz.CheckSwapScript(redeemScript, preimageHash, priv, response.TimeoutBlockHeight)
	if err != nil {
		fmt.Printf("Error checking swap script %v\n", err)
		return
	}

	err = boltz.CheckSwapAddress(b.ChainParams, response.Address, redeemScript, true)
	if err != nil {
		fmt.Printf("Error checking swap address %v\n", err)
		return
	}

	l := b.Lightning()
	if l == nil {
		fmt.Printf("Error checking lightning\n")
		return
	}
	defer l.Cleanup()
	resp, err := l.GetInfo(context.Background())
	if err != nil || resp.BlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		fmt.Printf("Error checking blockheight\n")
		return
	}

	fmt.Printf("ID %v\n", response.Id)

}
