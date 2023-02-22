// //go:build plugins

package boltz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/lightning"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
)

const FailNoCredsBoltz = false

func getAPI(t *testing.T, name string, typ api.APIType) agent_entities.NewAPICall {
	var data common_entities.Data

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		if FailNoCredsBoltz {
			t.Fatalf("No credentials")
		}
		return nil
	}

	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return nil
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return nil
	}

	s := strings.Split(data.Endpoint, ":")
	if len(s) == 2 {
		if typ == api.LndGrpc {
			data.Endpoint = fmt.Sprintf("%s:%s", s[0], "10009")
		} else if typ == api.LndRest {
			data.Endpoint = fmt.Sprintf("%s:%s", s[0], "8080")
		}
	}

	return func() lightning.LightingAPICalls {
		api := api.NewAPI(typ, func() (*common_entities.Data, error) {
			return &data, nil
		})

		return api
	}
}

func TestEnsureConnected(t *testing.T) {
	b := NewBoltzPlugin(getAPI(t, "fixture.secret", api.LndRest), "mainnet")
	if b == nil {
		if FailNoCredsBoltz {
			t.Fatalf("no credentials")
		}
		return
	}

	err := b.EnsureConnected(context.Background())
	assert.NoError(t, err)

	//b.Check("ITTpbl")
	//b.Swap()
	t.Fail()
}
