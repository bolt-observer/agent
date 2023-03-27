//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/plugins"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
	"github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli"
)

const (
	Name            = "boltz"
	DefaultBoltzUrl = "https://boltz.exchange/api"
	SecretBitSize   = 256
	SecretDbKey     = "secret"

	ErrInvalidArguments = Error("invalid arguments")
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
		Name: "maxfeepercentage", Value: 5.0, Usage: "maximum fee in percentage that is still acceptable", Hidden: false,
	},
	cli.Uint64Flag{
		Name: "maxswapsats", Value: 10_000_000, Usage: "maximum swap to perform in sats", Hidden: false,
	},
	cli.Uint64Flag{
		Name: "minswapsats", Value: 100_000, Usage: "minimum swap to perform in sats", Hidden: false,
	},
	cli.Uint64Flag{
		Name: "defaultswapsats", Value: 100_000, Usage: "default swap to perform in sats", Hidden: false,
	},
	cli.BoolFlag{
		Name: "disablezeroconf", Usage: "disable zeroconfirmation for swaps", Hidden: false,
	},
	cli.IntFlag{
		Name: "maxswapattempts", Value: 3, Usage: "max swap attempts for bigger jobs", Hidden: true,
	},
}

func init() {
	// Register ourselves with plugins
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, PluginFlags...)
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: Name,
		Init: func(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error) {
			r, err := NewPlugin(lnAPI, filter, cmdCtx, nodeDataInvalidator)
			return agent_entities.Plugin(r), err
		},
	})
}

// Plugin can save its data here
type Plugin struct {
	BoltzAPI            *boltz.Boltz
	ChainParams         *chaincfg.Params
	LnAPI               agent_entities.NewAPICall
	Filter              filter.FilteringInterface
	MaxFeePercentage    float64
	CryptoAPI           *CryptoAPI
	SwapMachine         *SwapMachine
	Redeemer            *Redeemer[FsmIn]
	ReverseRedeemer     *Redeemer[FsmIn]
	Limits              *SwapLimits
	NodeDataInvalidator agent_entities.Invalidatable
	isDryRun            bool
	db                  DB
	jobs                map[int32]interface{}
	mutex               sync.Mutex
	agent_entities.Plugin
}

type JobModel struct {
	ID   int32 `boltholdUnique:"UniqueID"`
	Data []byte
}

type Entropy struct {
	Data []byte
}

type SwapLimits struct {
	AllowZeroConf bool
	MinSwap       uint64
	MaxSwap       uint64
	DefaultSwap   uint64
	MaxAttempts   int
}

// NewPlugin creates new instance
func NewPlugin(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (*Plugin, error) {
	if lnAPI == nil {
		return nil, ErrInvalidArguments
	}

	db := &BoltzDB{}
	dbFile := agent_entities.CleanAndExpandPath(cmdCtx.String("boltzdatabase"))
	dir := path.Dir(dbFile)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		// ignore error on purpose
		os.Mkdir(dir, 0o640)
	}

	err := db.Connect(dbFile)
	if err != nil {
		return nil, err
	}

	entropy, err := setMnemonic(cmdCtx, db)
	if err != nil {
		return nil, err
	}

	resp := &Plugin{
		ChainParams: getChainParams(cmdCtx),
		BoltzAPI: &boltz.Boltz{
			URL: cmdCtx.String("boltzurl"),
		},
		CryptoAPI:           NewCryptoAPI(entropy),
		Filter:              filter,
		LnAPI:               lnAPI,
		NodeDataInvalidator: nodeDataInvalidator,
		db:                  db,
		jobs:                make(map[int32]interface{}),
		isDryRun:            cmdCtx.Bool("dryrun"),
	}
	resp.MaxFeePercentage = cmdCtx.Float64("maxfeepercentage")
	resp.BoltzAPI.Init(Btc) // required

	limits := &SwapLimits{
		AllowZeroConf: !cmdCtx.Bool("disablezeroconf"),
		MinSwap:       cmdCtx.Uint64("minswapsats"),
		MaxSwap:       cmdCtx.Uint64("maxswapsats"),
		DefaultSwap:   cmdCtx.Uint64("defaultswapsats"),
		MaxAttempts:   cmdCtx.Int("maxswapattempts"),
	}
	resp.Limits = limits

	// Swap machine is the finite state machine for doing the swap
	resp.SwapMachine = NewSwapMachine(resp, nodeDataInvalidator, JobDataToSwapData, resp.LnAPI)

	// Currently there is just one redeemer instance (perhaps split it)
	resp.Redeemer = NewRedeemer(context.Background(), (RedeemForward | RedeemReverse), resp.ChainParams, NewBoltzOnChainCommunicator(resp.BoltzAPI), resp.LnAPI,
		resp.getSleepTime(), resp.CryptoAPI, resp.SwapMachine.RedeemedCallback)
	resp.ReverseRedeemer = resp.Redeemer // use reference to same instance

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.CryptoAPI.DumpMnemonic())
	}

	return resp, nil
}

func (b *Plugin) getSleepTime() time.Duration {
	interval := 5 * time.Second
	if b.ChainParams.Name == "mainnet" {
		interval = 1 * time.Minute
	}
	return interval
}

func (b *Plugin) handleError(jobID int32, msgCallback agent_entities.MessageCallback, err error) error {
	glog.Infof("[Boltz] [%v] Error %v", jobID, err)
	if msgCallback != nil {
		msgCallback(agent_entities.PluginMessage{
			JobID:      int32(jobID),
			Message:    fmt.Sprintf("Error %v", err),
			IsError:    true,
			IsFinished: true,
		})
	}
	return err
}

func (b *Plugin) Execute(jobID int32, data []byte, msgCallback agent_entities.MessageCallback) error {
	var err error

	ctx := context.Background()

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, ok := b.jobs[jobID]; ok {
		// job already exists and is running already
		return nil
	} else {
		var sd SwapData

		if err = b.db.Get(jobID, &sd); err != nil {
			// create new job
			jd, err := ParseJobData(jobID, data)
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}

			lnAPI, err := b.LnAPI()
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}
			defer lnAPI.Cleanup()

			data, err := JobDataToSwapData(ctx, b.Limits, jd, msgCallback, lnAPI, b.Filter)
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}

			data.IsDryRun = b.isDryRun
			sd = *data
			sd.JobID = JobID(jobID)
			b.db.Insert(jobID, sd)
			b.jobs[jobID] = sd
		} else {
			// job found in database
			b.jobs[jobID] = sd
		}

		go b.runJob(jobID, &sd, msgCallback)
	}

	return nil
}

// start or continue running job
func (b *Plugin) runJob(jobID int32, jd *SwapData, msgCallback agent_entities.MessageCallback) {
	in := FsmIn{
		SwapData:    jd,
		MsgCallback: msgCallback,
	}

	// Running the job just means going through the state machine starting with jd.State
	if b.SwapMachine != nil {
		b.SwapMachine.Eval(in, jd.State)
	}
}

func getChainParams(cmdCtx *cli.Context) *chaincfg.Params {
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

	return &params
}

func setMnemonic(cmdCtx *cli.Context, db *BoltzDB) ([]byte, error) {
	var (
		entropy []byte
		dummy   Entropy
	)

	mnemonic := cmdCtx.String("setmnemonic")
	if mnemonic != "" {
		entropy, err := bip39.MnemonicToByteArray(mnemonic, true)
		if err != nil {
			return entropy, err
		}

		if err = db.Get(SecretDbKey, &dummy); err != nil {
			err = db.Insert(SecretDbKey, &Entropy{Data: entropy})
		} else {
			err = db.Update(SecretDbKey, &Entropy{Data: entropy})
		}
		if err != nil {
			return entropy, err
		}
	} else {
		err := db.Get(SecretDbKey, &dummy)
		entropy = dummy.Data

		if err != nil {
			entropy, err = bip39.NewEntropy(SecretBitSize)
			if err != nil {
				return entropy, err
			}
			err = db.Insert(SecretDbKey, &Entropy{Data: entropy})
			if err != nil {
				return entropy, err
			}
		}
	}
	return entropy, nil
}
