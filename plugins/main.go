//go:build plugins
// +build plugins

package plugins

import (
	"fmt"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/golang/glog"
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
type InitPluginFn func(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error)

// PluginData structure
type PluginData struct {
	Name string
	Init InitPluginFn
}

func InitPlugins(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) error {
	Plugins = make(map[string]agent_entities.Plugin)

	for _, p := range RegisteredPlugins {
		plugin, err := p.Init(lnAPI, filter, cmdCtx, nodeDataInvalidator)
		if err != nil {
			glog.Warningf("Plugin %s had an error %v\n", p.Name, err)
			return err
		}
		if plugin == nil {
			return fmt.Errorf("%s plugin failed to initialize", p.Name)
		}
		Plugins[p.Name] = plugin
	}

	return nil
}
