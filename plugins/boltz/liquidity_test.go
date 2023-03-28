//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNodeLiquidity(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	l := getAPI(t, "fixture.secret", api.LndGrpc)
	if l == nil {
		return
	}
	lnAPI, err := l()
	require.NoError(t, err)

	_, err = GetNodeLiquidity(context.Background(), lnAPI, f)
	require.NoError(t, err)
}

func TestGetByDescendingOutboundLiqudity(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	l := getAPI(t, "fixture.secret", api.LndGrpc)
	if l == nil {
		return
	}
	lnAPI, err := l()
	require.NoError(t, err)

	const Limit = uint64(3523220)
	resp, err := GetByDescendingOutboundLiquidity(context.Background(), Limit, lnAPI, f)
	require.NoError(t, err)

	total := uint64(0)
	for _, one := range resp {
		total += one.Capacity
	}

	require.GreaterOrEqual(t, total, Limit)
}
