//go:build plugins
// +build plugins

package boltz

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/tyler-smith/go-bip39"

	"github.com/golang/glog"
	"github.com/urfave/cli"
)

var PluginCommands = []cli.Command{
	{
		Name:  "boltz",
		Usage: "interact with boltz plugin",
		Subcommands: []cli.Command{
			{
				Name:   "submarineswap",
				Usage:  "invoke exactly one submarine swap aka swap-in (on-chain -> off-chain)",
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
				Usage:  "invoke exactly one reverse submarine swap aka swap-out (off-chain -> on-chain)",
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
			{
				Name:   "generaterefund",
				Usage:  "generate refund file to be used directly on boltz exchange GUI",
				Action: generateRefund,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name: "id", Value: 0, Usage: "id", Hidden: false,
					},
					cli.Int64Flag{
						Name: "attempt", Value: 0, Usage: "attempt", Hidden: false,
					},
					cli.StringFlag{
						Name: "file", Value: "", Usage: "file to write", Hidden: false,
					},
				},
			},
			{
				Name:   "dumpmnemonic",
				Usage:  "print master secret as mnemonic phrase (dangerous)",
				Action: dumpMnemonic,
			},
			{
				Name:   "setmnemonic",
				Usage:  "set a new secret from mnemonic phrase (dangerous)",
				Action: setNewMnemonic,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "mnemonic", Value: "", Usage: "mnemonic phrase (12 words)", Hidden: false,
					},
				},
			},
		},
	},
}

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
			FeesSoFar:        common.Fees{},
			FeesPending:      common.Fees{},
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
			FeesSoFar:        common.Fees{},
			FeesPending:      common.Fees{},
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

type RefundData struct {
	ID                 string `json:"id"`
	Currency           string `json:"currency"`
	RedeemScript       string `json:"redeemScript"`
	PrivateKey         string `json:"privateKey"`
	TimeoutBlockHeight uint32 `json:"timeoutBlockHeight"`
}

func generateRefund(c *cli.Context) error {
	var sd common.SwapData

	plugin, err := getPlugin(c)
	if err != nil {
		return err
	}

	id := int32(c.Int64("id"))
	if id == 0 {
		return fmt.Errorf("specify id")
	}

	attempt := c.Int64("attempt")

	if err = plugin.db.Get(id, &sd); err != nil {
		return err
	}

	if attempt == 0 {
		attempt = int64(sd.Attempt)
	}

	if sd.BoltzID == "" || sd.Script == "" {
		return fmt.Errorf("unknown swap")
	}

	k, err := plugin.CryptoAPI.GetKeys(sd.GetUniqueJobID())
	if err != nil {
		return err
	}

	r := RefundData{
		ID:                 sd.BoltzID,
		Currency:           common.Btc,
		RedeemScript:       sd.Script,
		PrivateKey:         hex.EncodeToString(k.Keys.PublicKey.SerializeCompressed()),
		TimeoutBlockHeight: sd.TimoutBlockHeight,
	}

	data, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}

	if c.String("file") == "" {
		fmt.Printf("%s\n", string(data))
	} else {
		err = ioutil.WriteFile(c.String("file"), data, 0644)
		if err != nil {
			return err
		}
	}

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

func dumpMnemonic(c *cli.Context) error {
	plugin, err := getPlugin(c)
	if err != nil {
		return err
	}

	fmt.Printf("Your mnemonic phrase is: %s\n", plugin.CryptoAPI.DumpMnemonic())
	return nil
}

func setNewMnemonic(c *cli.Context) error {
	var (
		entropy []byte
		dummy   Entropy
	)

	plugin, err := getPlugin(c)
	if err != nil {
		return err
	}

	mnemonic := c.String("mnemonic")
	if mnemonic == "" {
		return fmt.Errorf("mnemonic missing")
	}

	entropy, err = bip39.MnemonicToByteArray(mnemonic, true)
	if err != nil {
		return err
	}

	if err = plugin.db.Get(SecretDbKey, &dummy); err != nil {
		err = plugin.db.Insert(SecretDbKey, &Entropy{Data: entropy})
	} else {
		err = plugin.db.Update(SecretDbKey, &Entropy{Data: entropy})
	}
	if err != nil {
		return err
	}

	return nil
}
