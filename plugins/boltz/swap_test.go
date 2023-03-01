package boltz

import (
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/stretchr/testify/require"
)

func TestDoSwap(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	require.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx())
	if b == nil || b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}

	b.Swap("1337")

	t.Fail()
}
