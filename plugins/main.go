package plugins

import (
	"fmt"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/urfave/cli"
)

// Here we have no dependency on the actual plugin

var (
	// Plugins is global map which holds registered plugins
	Plugins map[string]agent_entities.Plugin

	// Both of those are filled during init

	// AllPluginFlags hold the extra flags for plugins
	AllPluginFlags []cli.Flag
	// RegisteredPlugins
	RegisteredPlugins []PluginData
)

// InitPluginFn signature of init plugin function
type InitPluginFn func(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) agent_entities.Plugin

// PluginData structure
type PluginData struct {
	Name string
	Init InitPluginFn
}

func InitPlugins(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) error {
	Plugins = make(map[string]agent_entities.Plugin)

	for _, p := range RegisteredPlugins {
		plugin := p.Init(lnAPI, filter, cmdCtx)
		if plugin == nil {
			return fmt.Errorf("%s plugin failed to initialize", p.Name)
		}
		Plugins[p.Name] = plugin
	}

	return nil
}
