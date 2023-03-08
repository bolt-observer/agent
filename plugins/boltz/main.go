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
	Name            = "boltz"
	DefaultBoltzUrl = "https://boltz.exchange/api"
	SecretBitSize   = 256
	SecretDbKey     = "secret"

	DefaultSwap = 100_000
	MinSwap     = 50_001
	MaxSwap     = 1_000_000

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
}

func init() {
	// Register ourselves with plugins
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, PluginFlags...)
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: Name,
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
		CryptoAPI: NewCryptoAPI(entropy),
		Filter:    filter,
		LnAPI:     lnAPI,
		db:        db,
		jobs:      make(map[int32]interface{}),
	}
	resp.MaxFeePercentage = cmdCtx.Float64("maxfeepercentage")

	interval := 5 * time.Second
	if resp.ChainParams.Name == "mainnet" {
		interval = 1 * time.Minute
	}

	// Swap machine is the finite state machine for doing the swap
	resp.SwapMachine = NewSwapMachine(resp)

	// Currently there is just one redeemer instance (perhaps split it)
	resp.Redeemer = NewRedeemer(context.Background(), (RedeemForward | RedeemReverse), resp.ChainParams, resp.BoltzAPI, resp.LnAPI,
		interval, resp.CryptoAPI, resp.SwapMachine.RedeemedCallback)
	resp.ReverseRedeemer = resp.Redeemer // use reference to same instance

	if cmdCtx.Bool("dumpmnemonic") {
		fmt.Printf("Your secret is %s\n", resp.CryptoAPI.DumpMnemonic())
	}

	return resp, nil
}

func (b *Plugin) Execute(jobID int32, data []byte, msgCallback agent_entities.MessageCallback) error {
	ctx := context.Background()

	jd := &JobData{}
	err := json.Unmarshal(data, &jd)
	if err != nil {
		return ErrCouldNotParseJobData
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, ok := b.jobs[jobID]; ok {
		// job already exists and is running already
		return nil
	} else {
		var sd SwapData

		if err = b.db.Get(jobID, &sd); err != nil {
			// job found in database
			b.db.Insert(jobID, sd)
			b.jobs[jobID] = sd
		} else {
			// create new job
			data := b.jobDataToSwapData(ctx, jd, msgCallback)
			if data == nil {
				glog.Infof("[Boltz] [%v] Not supported job", jobID)
				msgCallback(agent_entities.PluginMessage{
					JobID:      int32(jobID),
					Message:    "Not supported job",
					IsError:    true,
					IsFinished: true,
				})
				return nil
			}

			sd = *data
			sd.JobID = JobID(jobID)
			b.db.Insert(jobID, sd)
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
	var entropy []byte

	mnemonic := cmdCtx.String("setmnemonic")
	if mnemonic != "" {
		entropy, err := bip39.MnemonicToByteArray(mnemonic, true)
		if err != nil {
			return entropy, err
		}
		err = db.Insert(SecretDbKey, &Entropy{Data: entropy})
		if err != nil {
			return entropy, err
		}
	} else {
		var e Entropy
		err := db.Get(SecretDbKey, &e)
		entropy = e.Data

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

func (b *Plugin) jobDataToSwapData(ctx context.Context, jobData *JobData, msgCallback agent_entities.MessageCallback) *SwapData {
	if jobData == nil {
		return nil
	}

	switch jobData.Target {
	case OutboundLiquidityNodePercent:
		liquidity := b.getLiquidity(ctx, jobData, msgCallback)
		if liquidity == nil {
			return nil
		}
		return b.convertLiqudityNodePercent(jobData, liquidity, msgCallback, true)
	case InboundLiquidityNodePercent:
		liquidity := b.getLiquidity(ctx, jobData, msgCallback)
		if liquidity == nil {
			return nil
		}
		return b.convertLiqudityNodePercent(jobData, liquidity, msgCallback, false)
	default:
		// Not supported yet
		return nil
	}
}

func (b *Plugin) getLiquidity(ctx context.Context, jobData *JobData, msgCallback agent_entities.MessageCallback) *Liquidity {
	liquidity, err := b.GetNodeLiquidity(ctx, nil)

	if err != nil {
		glog.Infof("[Boltz] [%d] Could not get liquidity", jobData.ID)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      int32(jobData.ID),
				Message:    "Could not get liquidity",
				IsError:    true,
				IsFinished: true,
			})
		}
		return nil
	}

	return liquidity
}

func (b *Plugin) convertLiqudityNodePercent(jobData *JobData, liquidity *Liquidity, msgCallback agent_entities.MessageCallback, outbound bool) *SwapData {
	val := liquidity.OutboundPercentage
	name := "outbound"
	if !outbound {
		val = liquidity.InboundPercentage
		name = "inbound"
	}

	if val > jobData.Percentage || jobData.Percentage < 0 || jobData.Percentage > 100 {
		glog.Infof("[Boltz] [%v] No need to do anything - current %s liquidity %v", jobData.ID, name, val)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      int32(jobData.ID),
				Message:    fmt.Sprintf("No need to do anything - current %s liquidity %v", name, val),
				IsError:    false,
				IsFinished: true,
			})
		}
		return nil
	}

	sats := float64(DefaultSwap)
	if liquidity.Capacity != 0 {
		factor := (jobData.Percentage - val) / float64(100)
		sats = float64(liquidity.Capacity) * factor

		sats = math.Min(math.Max(sats, MinSwap), MaxSwap)
	}

	s := &SwapData{
		JobID: JobID(jobData.ID),
		Sats:  uint64(math.Round(sats)),
	}

	if outbound {
		s.State = InitialForward
	} else {
		s.State = InitialReverse
	}

	return s
}
