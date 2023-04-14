//go:build !plugins
// +build !plugins

package plugins

import (
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/urfave/cli"
)

// Dummy file just to make "go vet" happpy

var (
	// AllPluginFlags hold the extra flags for plugins
	AllPluginFlags []cli.Flag
	// AllPluginCommands hold the extra commands for plugins
	AllPluginCommands []cli.Command
	Plugins           map[string]agent_entities.Plugin
	RegisteredPlugins []PluginData
)

// InitPluginFn signature of init plugin function
type InitPluginFn func(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) (agent_entities.Plugin, error)

// PluginData structure
type PluginData struct {
	Name string
	Init InitPluginFn
}

func InitPlugins(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) error {
	return nil
}
