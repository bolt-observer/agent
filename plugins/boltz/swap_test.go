package boltz

import (
	"context"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoSwap(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	require.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndGrpc), f, getMockCliCtx())
	if b == nil || b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}

	//b.Swap("1337")
	_, _, err = b.CalcFundsToReceive(context.Background(), false, 100000)
	assert.NoError(t, err)
	t.Fail()
}
