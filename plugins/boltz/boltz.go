// //go:build plugins

package boltz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	boltz "github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	plugins "github.com/bolt-observer/agent/plugins"
	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	Symbol   = "BTC"
	BoltzUrl = "https://boltz.exchange/api"
)

// BoltzPlugin struct.
type BoltzPlugin struct {
	BoltzAPI *boltz.Boltz
	plugins.Plugin
}

func NewBoltzPlugin(lightning entities.NewAPICall) *BoltzPlugin {
	resp := &BoltzPlugin{
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
