package boltz

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
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

	return func() (lightning.LightingAPICalls, error) {
		api, err := api.NewAPI(typ, func() (*common_entities.Data, error) {
			return &data, nil
		})

		return api, err
	}
}

func getMockCliCtx() *cli.Context {
	fs := &flag.FlagSet{}
	fs.String("boltzurl", DefaultBoltzUrl, "")
	fs.String("boltzdatabase", "/tmp/boltz.db", "")
	fs.String("network", "regtest", "")
	return cli.NewContext(nil, fs, nil)
}

func TestEnsureConnected(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx())
	if b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}
	assert.NoError(t, err)
	err = b.EnsureConnected(context.Background())
	assert.NoError(t, err)
}
