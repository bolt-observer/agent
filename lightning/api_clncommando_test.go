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

func TestFailureToConnect(t *testing.T) {

	// We don't want to connect to random IPs from CI, this actually crashed before the fix w/ api.Cleanup
	return

	/*
		api, err := NewAPI(ClnCommando, func() (*entities.Data, error) {
			return &entities.Data{
				PubKey:      "02bf4f3f91e4bad02fae3a6a55d711d41d62f317ca1c76294f76612bae4b102ad2",
				MacaroonHex: "SzPwo1iVvWaaaz_FR1z7Ds9PKo23P4zVkLjcNJb98to9MSZtZXRob2RebGlzdHxtZXRob2ReZ2V0fG1ldGhvZD1zdW1tYXJ5Jm1ldGhvZC9saXN0ZGF0YXN0b3Jl",
				Endpoint:    "1.2.3.4:9735",
			}, nil
		})
		assert.NoError(t, err)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
		defer cancel()
		api.GetInfo(ctx)

		api.Cleanup()
	*/
}
