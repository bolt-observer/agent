package boltz

import (
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/plugins"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
	"github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli"
)

const (
	DefaultBoltzUrl = "https://boltz.exchange/api"
	SecretBitSize   = 256
)

var PluginFlags = []cli.Flag{
	cli.StringFlag{
		Name: "boltzurl", Value: DefaultBoltzUrl, Usage: "url of boltz api", Hidden: false,
	},
	cli.BoolFlag{
		Name: "dumpmnemonic", Usage: "should we print master secret as mnemonic phrase (dangerous)", Hidden: false,
	},
	cli.StringFlag{
		Name: "setmnemonic", Value: "", Usage: "update saved secret with this key material (dangerous)", Hidden: false,
	},
}

func init() {
	// Register ourselves with plugins
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, PluginFlags...)
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: "boltz",
		Init: func(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) agent_entities.Plugin {
			r := NewPlugin(lnAPI, filter, cmdCtx)
			return agent_entities.Plugin(r)
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

	agent_entities.Plugin
}

// NewPlugin creates new instance
func NewPlugin(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) *Plugin {
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

	var (
		entropy []byte
		err     error
	)

	mnemonic := cmdCtx.String("setmnemonic")
	// TODO: save this to db
	if mnemonic != "" {
		entropy, err = bip39.MnemonicToByteArray(mnemonic)
		if err != nil {
			glog.Fatalf("Invalid mnemonic: %v", err)
		}
	} else {
		entropy, err = bip39.NewEntropy(SecretBitSize)
		if err != nil {
			glog.Fatalf("Bad entropy: %v", err)
		}
	}

	resp := &Plugin{
		ChainParams: &params,
		BoltzAPI: &boltz.Boltz{
			URL: cmdCtx.String("boltzurl"),
		},
		MasterSecret: entropy,
	}
	if lnAPI == nil {
		return nil
	}

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.DumpMnemonic())
	}

	resp.LnAPI = lnAPI
	return resp
}

// Execute is currently just mocked
func (b *Plugin) Execute(jobID int32, data []byte, msgCallback func(agent_entities.PluginMessage) error, isDryRun bool) error {
	go func() {
		// LOG
		<-time.After(2 * time.Second)
		glog.Infof("Swap executed, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap executed, waiting for confirmation",
			Data:       []byte(`{"fee": "123124"}`),
			IsError:    false,
			IsFinished: false,
		})

		// FINISHED
		<-time.After(4 * time.Second)
		glog.Infof("Swap finished, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap finished successfully",
			Data:       []byte(`{"swap_id": "123124"}`),
			IsError:    false,
			IsFinished: true,
		})
	}()

	// TODO
	// check in memory if job is already running
	// check DB if job is already running -> load to memory
	// create new job
	return nil
}
