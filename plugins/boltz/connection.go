//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"fmt"

	"github.com/bolt-observer/agent/lightning"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
)

// EnsureConnected - ensures node is connected with Boltz
func (b *Plugin) EnsureConnected(ctx context.Context, optLnAPI lightning.LightingAPICalls) error {
	nodes, err := b.BoltzAPI.GetNodes()
	if err != nil {
		return err
	}

	var lnAPI lightning.LightingAPICalls

	if optLnAPI != nil {
		lnAPI = optLnAPI
	} else {
		lnAPI, err = b.LnAPI()
		if err != nil {
			return err
		}
		if lnAPI == nil {
			return fmt.Errorf("could not get lightning api")
		}
		defer lnAPI.Cleanup()
	}

	node, hasNode := nodes.Nodes[common.Btc]

	if !hasNode {
		return fmt.Errorf("could not find Boltz LND node for symbol %s", common.Btc)
	}

	if len(node.URIs) == 0 {
		return fmt.Errorf("could not find URIs for Boltz LND node for symbol %s", common.Btc)
	}

	success := false
	last := ""

	for _, url := range node.URIs {
		err = lnAPI.ConnectPeer(ctx, url)
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
