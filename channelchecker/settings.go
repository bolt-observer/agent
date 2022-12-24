package channelchecker

import (
	"sync"
	"time"

	entities "github.com/bolt-observer/agent/entities"
)

// PerNodeSettings represents the settings per pubkey
type PerNodeSettings struct {
	mutex sync.RWMutex
	data  map[string]Settings
}

// NewPerNodeSettings creates PerNodeSettings
func NewPerNodeSettings() *PerNodeSettings {
	return &PerNodeSettings{
		mutex: sync.RWMutex{},
		data:  make(map[string]Settings),
	}
}

// GetKeys obtains all keys
func (s *PerNodeSettings) GetKeys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

// Get obtains settings by key
func (s *PerNodeSettings) Get(key string) Settings {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.data[key]
}

// Set sets new settings
func (s *PerNodeSettings) Set(key string, value Settings) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
}

// Delete deletes settings
func (s *PerNodeSettings) Delete(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
}

// Settings struct
type Settings struct {
	identifier     entities.NodeIdentifier
	settings       entities.ReportingSettings
	lastCheck      time.Time
	lastGraphCheck time.Time
	lastReport     time.Time
	callback       entities.BalanceReportCallback
	getAPI         entities.NewAPICall
}
