//go:build plugins
// +build plugins

package connection

import (
	"context"
	"fmt"

	"github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
)

// EnsureConnected - ensures node is connected with Boltz
func EnsureConnected(ctx context.Context, lnAPI lightning.LightingAPICalls, boltzAPI *bapi.BoltzPrivateAPI) error {
	nodes, err := boltzAPI.GetNodes()
	if err != nil {
		return err
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
