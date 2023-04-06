//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/stretchr/testify/require"
)

func TestEnsureConnected(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	require.NoError(t, err)

	b, err := NewPlugin(common.GetAPI(t, "fixture.secret", api.LndRest), f, common.GetMockCliCtx("", "", "regtest"), nil)
	if b == nil || b.LnAPI == nil {
		if common.FailNoCredsBoltz {
			t.Fail()
		}
		return
	}
	require.NoError(t, err)
	err = b.EnsureConnected(context.Background(), nil)
	require.NoError(t, err)
}
