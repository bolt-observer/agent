package nodeinfo

import (
	"sync"
	"time"

	entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
)

// GlobalSettings struct
type GlobalSettings struct {
	mutex sync.RWMutex
	data  map[string]Settings
}

// NewGlobalSettings - create a new GlobalSettings
func NewGlobalSettings() *GlobalSettings {
	return &GlobalSettings{
		mutex: sync.RWMutex{},
		data:  make(map[string]Settings),
	}
}

// GetKeys - get all keys
func (s *GlobalSettings) GetKeys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

// Get - get settings for one key
func (s *GlobalSettings) Get(key string) Settings {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.data[key]
}

// Set - set settings for one key
func (s *GlobalSettings) Set(key string, value Settings) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
}

// Delete - delete settings for one key
func (s *GlobalSettings) Delete(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
}

// Settings struct
type Settings struct {
	identifier entities.NodeIdentifier
	lastCheck  time.Time
	callback   entities.InfoCallback
	hash       uint64
	getAPI     entities.NewAPICall
	interval   entities.Interval
	private    bool
	filter     filter.FilteringInterface
}
