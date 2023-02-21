package boltz

import (
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/golang/glog"
)

// Plugin can save its data here
type Plugin struct {
	agent_entities.Plugin
}

// Execute is currently just mocked
func (b *Plugin) Execute(jobID int32, data []byte, msgCallback func(agent_entities.PluginMessage) error, isDryRun bool) error {
	go func() {
		// LOG
		<-time.After(2 * time.Second)
		glog.Infof("Swap executed, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap executed, waiting for confirmation",
			Data:       []byte(`{"fee": "123124"}`),
			IsError:    false,
			IsFinished: false,
		})

		// FINISHED
		<-time.After(4 * time.Second)
		glog.Infof("Swap finished, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap finished successfully",
			Data:       []byte(`{"swap_id": "123124"}`),
			IsError:    false,
			IsFinished: true,
		})
	}()

	// TODO
	// check in memory if job is already running
	// check DB if job is already running -> load to memory
	// create new job
	return nil
}
