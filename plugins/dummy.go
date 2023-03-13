//go:build !plugins
// +build !plugins

package plugins

import (
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/urfave/cli"
)

// Dummy file just to make "go vet" happpy

var (
	// AllPluginFlags hold the extra flags for plugins
	AllPluginFlags []cli.Flag
	Plugins        map[string]agent_entities.Plugin
)

func InitPlugins(lnAPI agent_entities.NewAPICall, filter filter.FilteringInterface, cmdCtx *cli.Context) error {
	return nil
}
