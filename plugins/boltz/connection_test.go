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
	"github.com/stretchr/testify/require"
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

func getMockCliCtx(boltzUrl string, dbFile string) *cli.Context {
	fs := &flag.FlagSet{}
	if boltzUrl == "" {
		fs.String("boltzurl", DefaultBoltzUrl, "")
	} else {
		fs.String("boltzurl", boltzUrl, "")
	}
	if dbFile == "" {
		dbFile = "/tmp/boltz.db"
	}

	fs.String("boltzdatabase", dbFile, "")
	fs.String("network", "regtest", "")
	fs.Float64("maxfeepercentage", 80.0, "")
	fs.Uint64("maxswapsats", 1_000_000, "")
	fs.Uint64("minswapsats", 100_000, "")
	fs.Uint64("defaultswapsats", 100_000, "")
	fs.Bool("disablezeroconf", false, "")

	return cli.NewContext(nil, fs, nil)
}

func TestEnsureConnected(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	require.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx("", ""))
	if b == nil || b.LnAPI == nil {
		if FailNoCredsBoltz {
			t.Fail()
		}
		return
	}
	require.NoError(t, err)
	err = b.EnsureConnected(context.Background(), nil)
	require.NoError(t, err)
}
