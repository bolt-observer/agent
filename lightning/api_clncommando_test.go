package lightning

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetViaCommando(t *testing.T) {
	api := getAPI(t, "fixture-cln.secret", ClnCommando)
	if api == nil {
		return
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	info, err := api.GetInfo(ctx)
	assert.NoError(t, err)
	assert.Equal(t, true, info.IsSyncedToChain)
	assert.Equal(t, true, info.IsSyncedToGraph)
	assert.Less(t, 0, len(info.Alias))

	full, err := api.GetNodeInfoFull(ctx, true, true)
	assert.NoError(t, err)

	assert.Equal(t, true, full.Node.Alias == info.Alias)
}
