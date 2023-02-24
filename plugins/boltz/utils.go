package boltz

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/tyler-smith/go-bip39"
)

const (
	// Symbol - we support only bitcoin
	Symbol = "BTC"
)

// EnsureConnected - ensures node is connected with Boltz
func (b *Plugin) EnsureConnected(ctx context.Context) error {
	nodes, err := b.BoltzAPI.GetNodes()
	if err != nil {
		return err
	}

	lapi := b.LnAPI()
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

func (b *Plugin) DumpMnemonic() string {
	mnemonic, err := bip39.NewMnemonic(b.MasterSecret)
	if err != nil {
		glog.Warningf("DumpMnemonic failed %v", err)
		return ""
	}

	return mnemonic
}
