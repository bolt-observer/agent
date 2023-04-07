//go:build plugins
// +build plugins

package connection

import (
	"context"
	"testing"

	api "github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/stretchr/testify/require"
)

func TestEnsureConnected(t *testing.T) {
	api := common.GetAPI(t, "fixture.secret", api.LndRest)
	if api == nil {
		return
	}
	lnAPI, err := api()
	require.NoError(t, err)
	err = EnsureConnected(context.Background(), lnAPI, bapi.NewBoltzPrivateAPI("http://127.0.0.1:9001", nil))
	require.NoError(t, err)
}
