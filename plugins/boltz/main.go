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
	"github.com/cenkalti/backoff/v4"
	"github.com/golang/glog"
	"github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli"
	"golang.org/x/exp/constraints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	SecretDbKey = "secret"
)

func GenFlags(prefix string) []cli.Flag {
	return []cli.Flag{
		cli.Float64Flag{
			Name: fmt.Sprintf("%smaxfeepercentage", prefix), Value: 5.0, Usage: "maximum fee in percentage that is still acceptable", Hidden: prefix != "",
		},
		cli.Float64Flag{
			Name: fmt.Sprintf("%sbackoffamount", prefix), Value: 0.8, Usage: "if invoice could not be paid try with backoffamount * original amount", Hidden: true,
		},
		cli.Uint64Flag{
			Name: fmt.Sprintf("%smaxswapsats", prefix), Value: 0, Usage: "maximum swap to perform in sats", Hidden: true,
		},
		cli.Uint64Flag{
			Name: fmt.Sprintf("%sminswapsats", prefix), Value: 0, Usage: "minimum swap to perform in sats", Hidden: true,
		},
		cli.Uint64Flag{
			Name: fmt.Sprintf("%sdefaultswapsats", prefix), Value: 0, Usage: "default swap to perform in sats", Hidden: true,
		},
		cli.BoolTFlag{
			Name: fmt.Sprintf("%szeroconf", prefix), Usage: "enable zero-confirmation for swaps", Hidden: prefix != "",
		},
		cli.IntFlag{
			Name: fmt.Sprintf("%smaxswapattempts", prefix), Value: 20, Usage: "maximum number of individual boltz swaps to bring liquidity to desired state", Hidden: true,
		},
	}
}

var GenericFlags = []cli.Flag{
	cli.BoolFlag{
		Name: "disableboltz", Usage: "Disable boltz swapping", Hidden: true,
	},
}

var BoltzFlags = []cli.Flag{
	cli.StringFlag{
		Name: "boltzurl", Value: "", Usage: fmt.Sprintf("url of boltz api - empty means default - %s or %s", common.DefaultBoltzUrl, common.DefaultBoltzTestnetUrl), Hidden: false,
	},
	cli.StringFlag{
		Name: "boltzdatabase", Value: btcutil.AppDataDir("bolt", false) + "/boltz.db", Usage: "full path to database file (file will be created if it does not exist yet)", Hidden: false,
	},
	cli.StringFlag{
		Name: "boltzreferral", Value: "bolt-observer", Usage: "boltz referral code", Hidden: true,
	},
	cli.BoolFlag{
		Name: "boltzopenchannel", Usage: "whether boltz should open channnels if it cannot pay invoice", Hidden: true,
	},
	cli.BoolTFlag{
		Name: "boltzbelowminsuccess", Usage: "whether when you want to swap an amount below min is treated as success", Hidden: true,
	},
}

func init() {
	// Register ourselves with plugins
	const Name = "boltz"

	plugins.AllPluginFlags = append(plugins.AllPluginFlags, GenericFlags...)
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, BoltzFlags...)
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, GenFlags("")...)
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, GenFlags(Name)...)

	plugins.AllPluginCommands = append(plugins.AllPluginCommands, GenCommand(Name))
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: Name,
		Init: func(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error) {
			if cmdCtx.Bool("disableboltz") {
				return nil, fmt.Errorf("plugin disabled")
			}
			r, err := NewPlugin(lnAPI, filter, cmdCtx, nodeDataInvalidator, Name)
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
	jobs                map[int64]interface{}
	mutex               sync.Mutex

	agent_entities.PluginCommon
}

type JobModel struct {
	ID   int64 `boltholdUnique:"UniqueID"`
	Data []byte
}

type Entropy struct {
	Data []byte
}

func maxIgnoreZero[T constraints.Ordered](s ...T) T {
	var zero T
	if len(s) == 0 {
		return zero
	}
	m := s[0]
	for _, v := range s {
		if m < v && v != zero {
			m = v
		}
	}
	return m
}

func minIgnoreZero[T constraints.Ordered](s ...T) T {
	var zero T
	if len(s) == 0 {
		return zero
	}
	m := s[0]
	for _, v := range s {
		if m > v && v != zero {
			m = v
		}
	}
	return m
}

// NewPlugin creates new instance
func NewPlugin(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable, prefix string) (*Plugin, error) {
	if lnAPI == nil {
		return nil, common.ErrInvalidArguments
	}

	db := &BoltzDB{}
	dbFile := agent_entities.CleanAndExpandPath(cmdCtx.String(fmt.Sprintf("%sdatabase", prefix)))
	dir := path.Dir(dbFile)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		// ignore error on purpose
		os.Mkdir(dir, 0o750)
	}

	err := db.Connect(dbFile)
	if err != nil {
		return nil, err
	}

	entropy, err := getEntropy(cmdCtx, db)
	if err != nil {
		return nil, err
	}

	url := ""

	switch prefix {
	case "boltz":
		url = common.DefaultBoltzUrl
		if cmdCtx.String("boltzurl") == "" {
			if cmdCtx.String("network") == "testnet" {
				url = common.DefaultBoltzTestnetUrl
			}
		} else {
			url = cmdCtx.String("boltzurl")
		}
	case "diamondhands":
		url = cmdCtx.String("diamondhandsurl")
		if url == "" {
			url = common.DefaultDiamondhandsUrl
		}
	default:
		return nil, fmt.Errorf("prefix %s not supported", prefix)
	}

	caser := cases.Title(language.AmericanEnglish)
	resp := &Plugin{
		BoltzAPI:            bapi.NewBoltzPrivateAPI(url, nil),
		Filter:              filter,
		ChainParams:         getChainParams(cmdCtx),
		CryptoAPI:           crypto.NewCryptoAPI(entropy),
		LnAPI:               lnAPI,
		NodeDataInvalidator: nodeDataInvalidator,
		db:                  db,
		jobs:                make(map[int64]interface{}),
		isDryRun:            cmdCtx.Bool("dryrun"),
	}

	resp.PluginName = caser.String(prefix)

	limits := common.SwapLimits{
		MaxFeePercentage:        minIgnoreZero(cmdCtx.Float64(fmt.Sprintf("%smaxfeepercentage", prefix)), cmdCtx.Float64("maxfeepercentage")),
		AllowZeroConf:           cmdCtx.BoolT("zeroconf") && cmdCtx.BoolT(fmt.Sprintf("%szeroconf", prefix)),
		MinSwap:                 minIgnoreZero(cmdCtx.Uint64(fmt.Sprintf("%sminswapsats", prefix)), cmdCtx.Uint64("minswapsats")),
		MaxSwap:                 maxIgnoreZero(cmdCtx.Uint64(fmt.Sprintf("%smaxswapsats", prefix)), cmdCtx.Uint64("maxswapsats")),
		DefaultSwap:             maxIgnoreZero(cmdCtx.Uint64(fmt.Sprintf("%sdefaultswapsats", prefix)), cmdCtx.Uint64("defaultswapsats")),
		MaxAttempts:             minIgnoreZero(cmdCtx.Int(fmt.Sprintf("%smaxswapattempts", prefix)), cmdCtx.Int("maxswapattempts")),
		BackOffAmount:           maxIgnoreZero(cmdCtx.Float64(fmt.Sprintf("%sbackoffamount", prefix)), cmdCtx.Float64("backoffamount")),
		BelowMinAmountIsSuccess: cmdCtx.BoolT(fmt.Sprintf("%sbelowminsuccess", prefix)),
	}

	if limits.MinSwap > limits.MaxSwap || limits.DefaultSwap < limits.MinSwap || limits.DefaultSwap > limits.MaxSwap {
		return nil, fmt.Errorf("invalid limits")
	}

	if limits.MaxAttempts < 0 {
		return nil, fmt.Errorf("invalid maxattempts")
	}

	// we don't check for negative values since fees can actually be negative with DH
	if limits.MaxFeePercentage >= 100 {
		return nil, fmt.Errorf("invalid maxfeepercentage")
	}

	if limits.BackOffAmount <= 0 || limits.BackOffAmount >= 1.0 {
		return nil, fmt.Errorf("invalid backoffamount")
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 3 * time.Second
	err = backoff.Retry(func() error {
		return fixLimits(&limits, resp.BoltzAPI)
	}, b)

	if err != nil {
		glog.Errorf("Could not get limits from Boltz API: %v, try specifying --minswapsats and --maxswapsats or make sure --%surl is valid", err, prefix)
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
			ReferralCode:    cmdCtx.String(fmt.Sprintf("%sreferral", prefix)),
			Redeemer:        resp.Redeemer,
			ReverseRedeemer: resp.Redeemer,
			Limits:          resp.Limits,
			ChangeStateFn:   data.ChangeStateFn(resp.changeState),
			GetSleepTimeFn: data.GetSleepTimeFn(func(in common.FsmIn) time.Duration {
				return resp.GetSleepTime()
			}),
			DeleteJobFn:         resp.DeleteJob,
			AllowChanCreation:   cmdCtx.Bool(fmt.Sprintf("%sopenchannel", prefix)),
			PrivateChanCreation: false,
			PluginName:          caser.String(prefix),
		}, nodeDataInvalidator, common.JobDataToSwapData, resp.LnAPI)

	resp.Redeemer.SetCallback(resp.SwapMachine.RedeemedCallback)

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

		if limits.DefaultSwap == 0 {
			limits.DefaultSwap = limits.MinSwap
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

func (b *Plugin) handleError(jobID int64, msgCallback agent_entities.MessageCallback, err error) error {
	if err == common.ErrNoNeedToDoAnything {
		glog.Infof("[%v] [%v] Nothing to do %v", b.PluginName, jobID, err)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobID,
				Message:    "No need to do anything",
				IsError:    false,
				IsFinished: true,
			})
		}
	} else {
		glog.Infof("[%v] [%v] Error %v", b.PluginName, jobID, err)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobID,
				Message:    fmt.Sprintf("Error %v", err),
				IsError:    true,
				IsFinished: true,
			})
		}
	}
	return err
}

func (b *Plugin) DeleteJob(jobID int64) error {
	var sd common.SwapData

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if err := b.db.Get(jobID, &sd); err == nil {
		if _, ok := b.jobs[jobID]; ok {
			delete(b.jobs, jobID)
		}

		return b.db.Delete(jobID, sd)
	} else {
		return err
	}
}

func (b *Plugin) Execute(jobID int64, data []byte, msgCallback agent_entities.MessageCallback) error {
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

			data, err := common.JobDataToSwapData(ctx, b.Limits, jd, msgCallback, lnAPI, b.Filter, b.PluginName)
			if err != nil {
				return b.handleError(jobID, msgCallback, err)
			}

			data.Name = b.PluginName
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
func (b *Plugin) runJob(jobID int64, jd *common.SwapData, msgCallback agent_entities.MessageCallback) common.FsmOut {
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

func getEntropy(cmdCtx *cli.Context, db *BoltzDB) ([]byte, error) {
	var (
		entropy []byte
		dummy   Entropy
	)

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

	return entropy, nil
}
