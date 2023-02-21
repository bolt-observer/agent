package plugins

import (
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/plugins/boltz"
)

// Plugins is global map which holds registred plugins
var Plugins map[string]agent_entities.Plugin

func init() {
	Plugins = map[string]agent_entities.Plugin{
		"boltz": &boltz.Plugin{},
	}
}
