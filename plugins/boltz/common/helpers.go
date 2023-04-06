//go:build plugins
// +build plugins

package common

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bolt-observer/agent/lightning"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/urfave/cli"
)

const (
	FailNoCredsBoltz = false
	DefaultBoltzUrl  = "https://boltz.exchange/api"
	BtcPair          = "BTC/BTC"
	Btc              = "BTC"
	SecretBitSize    = 256
)

func GetAPI(t *testing.T, name string, typ api.APIType) api.NewAPICall {
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

func GetMockCliCtx(boltzUrl string, dbFile string, network string) *cli.Context {
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
	fs.String("network", network, "")
	fs.Float64("maxfeepercentage", 80.0, "")
	fs.Uint64("maxswapsats", 1_000_000, "")
	fs.Uint64("minswapsats", 100_000, "")
	fs.Uint64("defaultswapsats", 100_000, "")
	fs.Bool("disablezeroconf", false, "")
	fs.Uint64("maxswapattempts", 1, "")
	fs.String("boltzreferral", "bolt-observer", "")

	return cli.NewContext(nil, fs, nil)
}
