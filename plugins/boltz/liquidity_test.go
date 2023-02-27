package boltz

import (
	"fmt"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeLiquidity(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	b := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx())
	if b == nil {
		if FailNoCredsBoltz {
			t.Fatalf("no credentials")
		}
		return
	}

	resp, err := b.GetNodeLiquidity()
	assert.NoError(t, err)

	fmt.Printf("%+v\n", resp)
	t.Fail()
}
