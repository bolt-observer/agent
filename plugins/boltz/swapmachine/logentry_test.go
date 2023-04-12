//go:build plugins
// +build plugins

package swapmachine

import (
	"testing"

	"github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	sd := &common.SwapData{JobID: 1}

	l := NewLogEntry(sd)
	json := l.Get()
	assert.NotNil(t, json)
	t.Logf(string(json))

	json = l.Get("meso", "rdece", "sats", 3)
	assert.NotNil(t, json)
	t.Logf(string(json))
}
