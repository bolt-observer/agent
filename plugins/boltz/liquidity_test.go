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

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx())
	assert.NoError(t, err)

	_, err = b.GetNodeLiquidity(context.Background())
	assert.NoError(t, err)
}
