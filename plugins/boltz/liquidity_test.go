package boltz

import (
	"context"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeLiquidity(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndGrpc), f, getMockCliCtx())
	if b == nil || b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}
	assert.NoError(t, err)

	_, err = b.GetNodeLiquidity(context.Background(), nil)
	assert.NoError(t, err)
}

func TestGetByDescendingOutboundLiqudity(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndGrpc), f, getMockCliCtx())
	if b == nil || b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}
	assert.NoError(t, err)

	const Limit = uint64(3523220)
	resp, err := b.GetByDescendingOutboundLiquidity(context.Background(), Limit, nil)
	assert.NoError(t, err)

	total := uint64(0)
	for _, one := range resp {
		total += one.Capacity
	}

	assert.GreaterOrEqual(t, total, Limit)
}
