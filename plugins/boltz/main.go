//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/bolt-observer/agent/plugins"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	data "github.com/bolt-observer/agent/plugins/boltz/data"
	redeemer "github.com/bolt-observer/agent/plugins/boltz/redeemer"
	swapmachine "github.com/bolt-observer/agent/plugins/boltz/swapmachine"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
	"github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli"
)

const (
	Name        = "boltz"
	SecretDbKey = "secret"
)

var PluginFlags = []cli.Flag{
	cli.StringFlag{
		Name: "boltzurl", Value: "", Usage: "url of boltz api - empty means default", Hidden: false,
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
	cli.StringFlag{
		Name: "boltzreferral", Value: "bolt-observer", Usage: "boltz referral code", Hidden: true,
	},
}

var PluginCommands = []cli.Command{
	{
		Name:  "boltz",
		Usage: "interact with boltz plugin",
		Subcommands: []cli.Command{
			{
				Name:   "submarineswap",
				Usage:  "invoke submarine swap aka swap-in (on-chain -> off-chain)",
				Action: swap,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name: "id", Value: 0, Usage: "id", Hidden: true,
					},
					cli.Uint64Flag{
						Name: "sats", Value: 0, Usage: "satoshis to swap", Hidden: false,
					},
				},
			},
			{
				Name:   "reversesubmarineswap",
				Usage:  "invoke reverse submarine swap aka swap-out (off-chain -> on-chain)",
				Action: reverseSwap,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name: "id", Value: 0, Usage: "id", Hidden: true,
					},
					cli.Uint64Flag{
						Name: "sats", Value: 0, Usage: "satoshis to swap", Hidden: false,
					},
					cli.Uint64Flag{
						Name: "channelid", Value: 0, Usage: "channel id to use", Hidden: false,
					},
				},
			},
		},
	},
}

func init() {
	// Register ourselves with plugins
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, PluginFlags...)
	plugins.AllPluginCommands = append(plugins.AllPluginCommands, PluginCommands...)
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: Name,
		Init: func(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error) {
			r, err := NewPlugin(lnAPI, filter, cmdCtx, nodeDataInvalidator)
			return agent_entities.Plugin(r), err
		},
	})
}

// Plugin can save its data here
type Plugin struct {
	BoltzAPI            *bapi.BoltzPrivateAPI
	Filter              filter.FilteringInterface
	ChainParams         *chaincfg.Params
	LnAPI               lightning.NewAPICall
	CryptoAPI           *crypto.CryptoAPI
	SwapMachine         *swapmachine.SwapMachine
	Redeemer            *redeemer.Redeemer[common.FsmIn]
	Limits              common.SwapLimits
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

// NewPlugin creates new instance
func NewPlugin(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (*Plugin, error) {
	if lnAPI == nil {
		return nil, common.ErrInvalidArguments
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

	url := common.DefaultBoltzUrl
	if cmdCtx.String("boltzurl") == "" {
		if cmdCtx.String("network") == "testnet" {
			url = common.DefaultBoltzTestnetUrl
		}
	} else {
		url = cmdCtx.String("boltzurl")
	}

	resp := &Plugin{
		BoltzAPI:            bapi.NewBoltzPrivateAPI(url, nil),
		Filter:              filter,
		ChainParams:         getChainParams(cmdCtx),
		CryptoAPI:           crypto.NewCryptoAPI(entropy),
		LnAPI:               lnAPI,
		NodeDataInvalidator: nodeDataInvalidator,
		db:                  db,
		jobs:                make(map[int32]interface{}),
		isDryRun:            cmdCtx.Bool("dryrun"),
	}

	limits := common.SwapLimits{
		MaxFeePercentage: cmdCtx.Float64("maxfeepercentage"),
		AllowZeroConf:    !cmdCtx.Bool("disablezeroconf"),
		MinSwap:          cmdCtx.Uint64("minswapsats"),
		MaxSwap:          cmdCtx.Uint64("maxswapsats"),
		DefaultSwap:      cmdCtx.Uint64("defaultswapsats"),
		MaxAttempts:      cmdCtx.Int("maxswapattempts"),
	}

	err = fixLimits(&limits, resp.BoltzAPI)
	if err != nil {
		return nil, err
	}

	resp.Limits = limits

	// Currently there is just one redeemer instance (perhaps split it)
	resp.Redeemer = redeemer.NewRedeemer[common.FsmIn](context.Background(), (redeemer.RedeemForward | redeemer.RedeemReverse), resp.ChainParams,
		redeemer.NewBoltzOnChainCommunicator(resp.BoltzAPI), resp.LnAPI,
		resp.GetSleepTime(), resp.CryptoAPI)

	// Swap machine is the finite state machine for doing the swap
	resp.SwapMachine = swapmachine.NewSwapMachine(
		data.PluginData{
			ChainParams:     resp.ChainParams,
			Filter:          filter,
			CryptoAPI:       resp.CryptoAPI,
			BoltzAPI:        resp.BoltzAPI,
			ReferralCode:    cmdCtx.String("boltzreferral"),
			Redeemer:        resp.Redeemer,
			ReverseRedeemer: resp.Redeemer,
			Limits:          resp.Limits,
			ChangeStateFn:   data.ChangeStateFn(resp.changeState),
			GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
				return resp.GetSleepTime()
			}),
		}, nodeDataInvalidator, common.JobDataToSwapData, resp.LnAPI)

	resp.Redeemer.SetCallback(resp.SwapMachine.RedeemedCallback)

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.CryptoAPI.DumpMnemonic())
	}

	return resp, nil
}

func fixLimits(limits *common.SwapLimits, api *bapi.BoltzPrivateAPI) error {
	// zero limits get resolved via boltz API
	if limits.MinSwap == 0 || limits.MaxSwap == 0 {
		pairs, err := api.GetPairs()
		if err != nil {
			return err
		}

		res, ok := pairs.Pairs[common.BtcPair]
		if !ok {
			return fmt.Errorf("pairs are not available")
		}

		if limits.MinSwap == 0 {
			limits.MinSwap = res.Limits.Minimal
		}

		if limits.MaxSwap == 0 {
			limits.MaxSwap = res.Limits.Maximal
		}
	}

	return nil
}

func (b *Plugin) GetSleepTime() time.Duration {
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
		var sd common.SwapData

		if err = b.db.Get(jobID, &sd); err != nil {
			// create new job
			jd, err := common.ParseJobData(jobID, data)
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}

			lnAPI, err := b.LnAPI()
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}
			defer lnAPI.Cleanup()

			data, err := common.JobDataToSwapData(ctx, b.Limits, jd, msgCallback, lnAPI, b.Filter)
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}

			data.IsDryRun = b.isDryRun
			sd = *data
			sd.JobID = common.JobID(jobID)
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
func (b *Plugin) runJob(jobID int32, jd *common.SwapData, msgCallback agent_entities.MessageCallback) common.FsmOut {
	in := common.FsmIn{
		SwapData:    jd,
		MsgCallback: msgCallback,
	}

	// Running the job just means going through the state machine starting with jd.State
	if b.SwapMachine != nil {
		resp := b.SwapMachine.Eval(in, jd.State)
		return resp
	}

	return common.FsmOut{}
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
			entropy, err = bip39.NewEntropy(common.SecretBitSize)
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

// The methods below are invoked via boltz command directly

func getRandomId() int32 {
	// Generates a random negative integer - so it does not clash with actions api
	n := int64(rand.Int31n(math.MaxInt32))
	x := n | (1 << 31)

	return int32(x)
}

func getPlugin(c *cli.Context) (*Plugin, error) {
	entities.GlogShim(c)

	parentCtx := c.Parent().Parent()
	// Need this so admin macaroon is used
	parentCtx.Set("actions", "true")

	f, err := filter.NewAllowAllFilter()
	if err != nil {
		return nil, err
	}

	plugin, err := NewPlugin(agent_entities.MkGetLndAPI(parentCtx), f, parentCtx, nil)
	if err != nil {
		return nil, err
	}

	return plugin, nil
}

func swap(c *cli.Context) error {
	var sd common.SwapData

	plugin, err := getPlugin(c)
	if err != nil {
		return err
	}

	id := int32(c.Int64("id"))
	if id == 0 {
		id = getRandomId()
	}

	glog.Infof("In case you need to resume job specify --id %d", id)

	if err = plugin.db.Get(id, &sd); err != nil {
		if c.Uint64("sats") == 0 {
			return fmt.Errorf("need to specify positive amount of satoshis to swap")
		}

		sd = common.SwapData{
			JobID:            common.JobID(id),
			Attempt:          1,
			Sats:             c.Uint64("sats"),
			ReverseChannelId: 0,
			OriginalJobData:  common.DummyJobData,
			FeesPaidSoFar:    0,
			SatsSwappedSoFar: 0,
			SwapLimits:       plugin.Limits,
			State:            common.InitialForward,
			IsDryRun:         plugin.isDryRun,
		}

		plugin.db.Insert(id, sd)
		plugin.jobs[id] = sd

	} else {
		if sd.State.ToSwapType() == common.Reverse {
			return fmt.Errorf("use reversesubmarineswap")
		}

		plugin.jobs[id] = sd
	}

	go func() {
		resp := plugin.runJob(int32(sd.JobID), &sd, nil)
		if resp.Error != nil {
			os.Exit(1)
		}
	}()
	waitForJob(plugin, id)

	return nil
}

func reverseSwap(c *cli.Context) error {
	var sd common.SwapData

	plugin, err := getPlugin(c)
	if err != nil {
		return err
	}

	id := int32(c.Int64("id"))
	if id == 0 {
		id = getRandomId()
	}

	glog.Infof("In case you need to resume job specify --id %d", id)

	if err = plugin.db.Get(id, &sd); err != nil {
		if c.Uint64("sats") == 0 {
			return fmt.Errorf("need to specify positive amount of satoshis to swap")
		}

		sd = common.SwapData{
			JobID:            common.JobID(id),
			Attempt:          1,
			Sats:             c.Uint64("sats"),
			ReverseChannelId: c.Uint64("channelid"),
			OriginalJobData:  common.DummyJobData,
			FeesPaidSoFar:    0,
			SatsSwappedSoFar: 0,
			SwapLimits:       plugin.Limits,
			State:            common.InitialReverse,
			IsDryRun:         plugin.isDryRun,
		}

		plugin.db.Insert(id, sd)
		plugin.jobs[id] = sd

	} else {
		if sd.State.ToSwapType() == common.Forward {
			return fmt.Errorf("use submarineswap")
		}

		plugin.jobs[id] = sd
	}

	go func() {
		resp := plugin.runJob(int32(sd.JobID), &sd, nil)
		if resp.Error != nil {
			os.Exit(1)
		}
	}()
	waitForJob(plugin, id)

	return nil
}

func waitForJob(plugin *Plugin, id int32) {
	for {
		x, ok := plugin.jobs[id].(common.SwapData)
		if !ok {
			break
		}
		if x.State.IsFinal() {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
