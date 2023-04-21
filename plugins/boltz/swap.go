//go:build plugins
// +build plugins

package boltz

import (
	"fmt"

	"github.com/bolt-observer/agent/entities"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
)

// changeState actually registers the state change
func (b *Plugin) changeState(in common.FsmIn, state common.State) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	in.SwapData.State = state

	_, ok := b.jobs[int64(in.SwapData.JobID)]
	if ok {
		b.jobs[int64(in.SwapData.JobID)] = *in.SwapData
		err := b.db.Update(in.SwapData.JobID, *in.SwapData)
		if err != nil && in.MsgCallback != nil {
			in.MsgCallback(entities.PluginMessage{
				JobID:      int64(in.GetJobID()),
				Message:    fmt.Sprintf("Could not change state to %v %v", state, err),
				IsError:    true,
				IsFinished: true,
			})
		}
		return err
	}

	return nil
}
