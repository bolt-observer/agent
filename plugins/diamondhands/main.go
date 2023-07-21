//go:build plugins
// +build plugins

package diamondhands

import (
	"fmt"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/bolt-observer/agent/plugins"
	"github.com/bolt-observer/agent/plugins/boltz"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/urfave/cli"
)

var GenericFlags = []cli.Flag{
	cli.BoolFlag{
		Name: "disablediamondhands", Usage: "Disable diamondhands swapping", Hidden: true,
	},
}

var DiamondHandsFlags = []cli.Flag{
	cli.StringFlag{
		Name: "diamondhandsurl", Value: "", Usage: fmt.Sprintf("url of diamondhandsurl api - empty means default - %s or %s", common.DefaultDiamondhandsUrl, common.DefaultDiamondhandsTestnetUrl), Hidden: false,
	},
	cli.StringFlag{
		Name: "diamondhandsdatabase", Value: btcutil.AppDataDir("bolt", false) + "/diamondhands.db", Usage: "full path to database file (file will be created if it does not exist yet)", Hidden: false,
	},
	cli.StringFlag{
		Name: "diamondhandsreferral", Value: "", Usage: "diamondhands referral code", Hidden: true,
	},
	cli.BoolFlag{
		Name: "diamondhandsopenchannel", Usage: "whether diamondhands should open channnels if it cannot pay invoice", Hidden: true,
	},
	cli.BoolTFlag{
		Name: "diamondhandsbelowminsuccess", Usage: "whether when you want to swap an amount below min is treated as success", Hidden: true,
	},
}

func init() {
	// Register ourselves with plugins
	const Name = "diamondhands"

	plugins.AllPluginFlags = append(plugins.AllPluginFlags, GenericFlags...)
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, DiamondHandsFlags...)
	plugins.AllPluginFlags = append(plugins.AllPluginFlags, boltz.GenFlags(Name)...)

	plugins.AllPluginCommands = append(plugins.AllPluginCommands, boltz.GenCommand(Name))
	plugins.RegisteredPlugins = append(plugins.RegisteredPlugins, plugins.PluginData{
		Name: Name,
		Init: func(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error) {
			if cmdCtx.Bool("disablediamondhands") {
				return nil, fmt.Errorf("plugin disabled")
			}
			r, err := boltz.NewPlugin(lnAPI, filter, cmdCtx, nodeDataInvalidator, Name)
			return agent_entities.Plugin(r), err
		},
	})
}
