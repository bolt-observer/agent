package plugins

import "github.com/bolt-observer/agent/entities"

// Plugin struct.
type Plugin struct {
	DryRun    bool
	Lightning entities.NewAPICall
}
