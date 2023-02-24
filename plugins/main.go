package plugins

import (
	"fmt"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/plugins/boltz"
	"github.com/urfave/cli"
)

var (
	// Plugins is global map which holds registred plugins
	Plugins map[string]agent_entities.Plugin
	// PluginFlags hold the extra flags for plugins
	PluginFlags []cli.Flag
)

func InitPlugins(lnAPI agent_entities.NewAPICall, cmdCtx *cli.Context) error {
	Plugins = make(map[string]agent_entities.Plugin)

	plugin := boltz.NewBoltzPlugin(lnAPI, cmdCtx)
	if plugin == nil {
		return fmt.Errorf("boltz plugin failed to initialize")
	}

	Plugins["boltz"] = plugin

	return nil
}

func init() {
	PluginFlags = append(PluginFlags, boltz.PluginFlags...)
}
