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
	Plugins        map[string]agent_entities.Plugin
)

func InitPlugins(lnAPI api.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context, nodeDataInvalidator agent_entities.Invalidatable) error {
	return nil
}
