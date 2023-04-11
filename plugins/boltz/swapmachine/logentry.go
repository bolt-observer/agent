//go:build plugins
// +build plugins

package swapmachine

import (
	"encoding/json"

	common "github.com/bolt-observer/agent/plugins/boltz/common"
)

// SwapType enum.
type SwapType string

const (
	Unknown SwapType = "unknown"
	Forward SwapType = "forward"
	Reverse SwapType = "reverse"
)

// Log entry
type LogEntry map[string]any

func NewLogEntry(sd *common.SwapData, typ SwapType) LogEntry {
	ret := make(map[string]any)

	ret["id"] = int(sd.JobID)
	if sd.BoltzID != "" {
		ret["boltz_id"] = sd.BoltzID
	}
	ret["swap_type"] = typ
	return ret
}

func (e LogEntry) Get(keys ...any) []byte {
	l := len(keys)
	if l != 0 {
		if l < 2 || l%2 != 0 {
			return nil
		}

		for i := 0; i < len(keys); i += 2 {
			s, ok := keys[i].(string)
			if !ok {
				continue
			}
			e[s] = keys[i+1]
		}
	}

	out, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil
	}

	return out
}
