package boltz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	DefaultBoltzUrl = "https://boltz.exchange/api"
	SecretBitSize   = 256
	SecretDbKey     = "secret"

	DefaultSwap = 1000000
	MinSwap     = 50000

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
	BoltzAPI         *boltz.Boltz
	ChainParams      *chaincfg.Params
	LnAPI            agent_entities.NewAPICall
	Filter           filter.FilteringInterface
	MaxFeePercentage float64
	CryptoAPI        *CryptoAPI
	SwapMachine      *SwapMachine
	Redeemer         *Redeemer[FsmIn]
	ReverseRedeemer  *Redeemer[FsmIn]
	db               DB
	jobs             map[int32]interface{}
	mutex            sync.Mutex
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
func NewPlugin(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) (*Plugin, error) {
	if lnAPI == nil {
		return nil, ErrInvalidArguments
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

	var entropy []byte

	mnemonic := cmdCtx.String("setmnemonic")
	if mnemonic != "" {
		entropy, err = bip39.MnemonicToByteArray(mnemonic, true)
		if err != nil {
			return nil, err
		}
		err = db.Insert(SecretDbKey, &Entropy{Data: entropy})
		if err != nil {
			return nil, err
		}
	} else {
		var e Entropy
		err = db.Get(SecretDbKey, &e)
		entropy = e.Data

		if err != nil {
			entropy, err = bip39.NewEntropy(SecretBitSize)
			if err != nil {
				return nil, err
			}
			err = db.Insert(SecretDbKey, &Entropy{Data: entropy})
			if err != nil {
				return nil, err
			}
		}
	}

	resp := &Plugin{
		ChainParams: &params,
		BoltzAPI: &boltz.Boltz{
			URL: cmdCtx.String("boltzurl"),
		},
		CryptoAPI: NewCryptoAPI(entropy),
		Filter:    filter,
		LnAPI:     lnAPI,
		db:        db,
	}

	interval := 5 * time.Second
	if params.Name == "mainnet" {
		interval = 1 * time.Minute
	}

	resp.SwapMachine = NewSwapMachine(resp)
	// Currently there is just one redeemer instance (perhaps split it)
	resp.Redeemer = NewRedeemer(context.Background(), (RedeemNormal | RedeemReverse), resp.ChainParams, resp.BoltzAPI, resp.LnAPI, interval, resp.CryptoAPI, resp.SwapMachine.RedeemerCallback)
	resp.ReverseRedeemer = resp.Redeemer

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.CryptoAPI.DumpMnemonic())
	}

	resp.MaxFeePercentage = cmdCtx.Float64("maxfeepercentage")

	return resp, nil
}

func (b *Plugin) jobDataToSwapData(jobData *JobData, msgCallback agent_entities.MessageCallback) *SwapData {
	if jobData == nil {
		return nil
	}

	switch jobData.Target {
	case OutboundLiqudityNodeTarget:
		return b.convertOutBoundLiqudityNodeTarget(jobData, msgCallback)
	default:
		// Not supported yet
		return nil
	}
}

func (b *Plugin) convertOutBoundLiqudityNodeTarget(jobData *JobData, msgCallback agent_entities.MessageCallback) *SwapData {
	liquidity, err := b.GetNodeLiquidity(context.Background(), nil)

	if err != nil {
		glog.Infof("[Boltz] [%d] Could not get liquidity", jobData.ID)
		msgCallback(agent_entities.PluginMessage{
			JobID:      int32(jobData.ID),
			Message:    "Could not get liquidity",
			IsError:    true,
			IsFinished: true,
		})
		return nil
	}

	if liquidity.OutboundPercentage > jobData.Percentage || jobData.Percentage < 0 || jobData.Percentage > 100 {
		glog.Infof("[Boltz] [%v] No need to do anything - current outbound liquidity %v", jobData.ID, liquidity.OutboundPercentage)
		msgCallback(agent_entities.PluginMessage{
			JobID:      int32(jobData.ID),
			Message:    fmt.Sprintf("No need to do anything - current outbound liquidity %v", liquidity.OutboundPercentage),
			IsError:    false,
			IsFinished: true,
		})
		return nil
	}

	target := float64(jobData.Percentage) / 100
	sats := float64(DefaultSwap)
	if liquidity.Capacity != 0 {
		sats = target / float64(liquidity.Capacity)
		sats -= float64(liquidity.OutboundSats)

		sats = math.Max(sats, MinSwap)
	}

	return &SwapData{
		JobID: JobID(jobData.ID),
		Sats:  uint64(math.Round(sats)),
		State: InitialNormal,
	}
}

func (b *Plugin) Execute(jobID int32, data []byte, msgCallback agent_entities.MessageCallback) error {
	jd := &JobData{}
	err := json.Unmarshal(data, &jd)
	if err != nil {
		return ErrCouldNotParseJobData
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// job already exists and is running already
	if _, ok := b.jobs[jobID]; ok {
		return nil
	} else {
		var sd SwapData
		if err = b.db.Get(jobID, &sd); err != nil {
			b.db.Insert(jobID, sd)
		}
		data := b.jobDataToSwapData(jd, msgCallback)
		if data == nil {
			glog.Infof("[Boltz] [%v] Non supported job", jobID)
			msgCallback(agent_entities.PluginMessage{
				JobID:      int32(jobID),
				Message:    "Non supported job",
				IsError:    true,
				IsFinished: true,
			})
			return nil
		}

		b.jobs[jobID] = *data
		go b.runJob(jobID, data, msgCallback)
	}

	return nil
}

// start or continue running job
func (b *Plugin) runJob(jobID int32, jd *SwapData, msgCallback agent_entities.MessageCallback) {
	in := FsmIn{
		SwapData:    *jd,
		MsgCallback: msgCallback,
	}

	b.SwapMachine.Eval(in, jd.State)
}
