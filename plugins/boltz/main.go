package boltz

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/plugins"
	"github.com/bolt-observer/agent/plugins/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli"
)

const (
	DefaultBoltzUrl = "https://boltz.exchange/api"
	SecretBitSize   = 256
	SecretDbKey     = "secret"
)

var PluginFlags = []cli.Flag{
	cli.StringFlag{
		Name: "boltzurl", Value: DefaultBoltzUrl, Usage: "url of boltz api", Hidden: false,
	},
	cli.StringFlag{
		Name: "boltzdatabase", Value: btcutil.AppDataDir("bolt", false) + "/boltz.db", Usage: "full path to database file (file will be created if it does not exist yet)", Hidden: false,
	},
	cli.BoolFlag{
		Name: "dumpmnemonic", Usage: "should we print master secret as mnemonic phrase (dangerous)", Hidden: false,
	},
	cli.StringFlag{
		Name: "setmnemonic", Value: "", Usage: "update saved secret with this key material (dangerous)", Hidden: false,
	},
	cli.Float64Flag{
		Name: "maxfeepercentage", Value: 5.0, Usage: "maximum fee that is still acceptable", Hidden: false,
	},
}

func init() {
	// Register ourselves with plugins
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, PluginFlags...)
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: "boltz",
		Init: func(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) (agent_entities.Plugin, error) {
			r, err := NewPlugin(lnAPI, filter, cmdCtx)
			return agent_entities.Plugin(r), err
		},
	})
}

// Plugin can save its data here
type Plugin struct {
	BoltzAPI     *boltz.Boltz
	ChainParams  *chaincfg.Params
	LnAPI        entities.NewAPICall
	Filter       filter.FilteringInterface
	MasterSecret []byte
	db           DB
	jobs         map[int32]interface{}
	mutex        sync.Mutex
	agent_entities.Plugin
}

type JobModel struct {
	ID   int32 `boltholdUnique:"UniqueID"`
	Data []byte
}

// NewPlugin creates new instance
func NewPlugin(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) (*Plugin, error) {
	fmt.Printf("New plugin\n")

	if lnAPI == nil {
		return nil, types.ErrInvalidArguments
	}

	network := cmdCtx.String("network")

	params := chaincfg.MainNetParams
	switch network {
	case "mainnet":
		params = chaincfg.MainNetParams
	case "testnet":
		params = chaincfg.TestNet3Params
	case "regtest":
		params = chaincfg.RegressionNetParams
	case "simnet":
		params = chaincfg.SimNetParams
	}

	db := &BoltzDB{}
	//dbFile := agent_entities.CleanAndExpandPath(cmdCtx.String("boltzdatabase"))
	err := db.Connect("/tmp/boltz.db")
	if err != nil {
		fmt.Printf("Fail %v\n", err)
		return nil, err
	}

	var entropy []byte

	mnemonic := cmdCtx.String("setmnemonic")
	if mnemonic != "" {
		entropy, err = bip39.MnemonicToByteArray(mnemonic, true)
		if err != nil {
			return nil, err
		}
	} else {
		err = db.Get(SecretDbKey, entropy)
		fmt.Printf("Got entropy: %v\n", entropy)

		if err != nil {
			entropy, err = bip39.NewEntropy(SecretBitSize)
			if err != nil {
				return nil, err
			}
			fmt.Printf("Set secret")
			db.Insert(SecretDbKey, entropy)
		}
	}

	resp := &Plugin{
		ChainParams: &params,
		BoltzAPI: &boltz.Boltz{
			URL: cmdCtx.String("boltzurl"),
		},
		MasterSecret: entropy,
		Filter:       filter,
		LnAPI:        lnAPI,
		db:           db,
	}

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.DumpMnemonic())
	}

	return resp, nil
}

func (b *Plugin) Execute(jobID int32, data []byte, msgCallback entities.MessageCallback) error {
	jd := &types.JobData{}
	err := json.Unmarshal(data, &jd)
	if err != nil {
		return types.ErrCouldNotParseJobData
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// job already exists and is running already
	if _, ok := b.jobs[jobID]; ok {
		return nil
	} else {
		var jm types.JobData
		if err = b.db.Get(jobID, &jm); err != nil {
			b.db.Insert(jobID, jd)
		}
		b.jobs[jobID] = struct{}{}

		go b.runJob(jobID, jd, msgCallback)
	}

	return nil
}

// start or continue running job
func (b *Plugin) runJob(jobID int32, jd *types.JobData, msgCallback entities.MessageCallback) {
	// switch jd.Target and do appropriate swaps
}
