package nodedata

import (
	"sync"

	"github.com/golang/glog"
)

// ChannelCache struct.
type ChannelCache interface {
	// Lock - acquire lock
	Lock()
	// Unlock - release lock
	Unlock()
	// Get - get channel cache
	Get(name string) (string, bool)
	// Set - set channel cache
	Set(name string, value string)
	// DeferredSet - set channel cache but do not commit it yet
	DeferredSet(name, old, new string)
	// DeferredCommit - commit channel cache from DeferredSet
	DeferredCommit() bool
	// DeferredRevert - revert channel cache from DeferredSet
	DeferredRevert() bool
}

// Lock - acquire lock.
func (c *InMemoryChannelCache) Lock() {
	c.chanMutex.Lock()
}

// Unlock - release lock.
func (c *InMemoryChannelCache) Unlock() {
	c.chanMutex.Unlock()
}

// Get - get channel cache.
func (c *InMemoryChannelCache) Get(name string) (string, bool) {
	a, b := c.chanCache[name]
	return a, b
}

// Set - set channel cache.
func (c *InMemoryChannelCache) Set(name string, value string) {
	c.chanCache[name] = value
}

// OldNewVal struct.
type OldNewVal struct {
	OldValue string
	NewValue string
}

// InMemoryChannelCache - ChannelCache implementation that stores data in memory.
type InMemoryChannelCache struct {
	chanMutex sync.Mutex
	chanCache map[string]string

	deferredCacheMutex sync.Mutex
	deferredCache      map[string]OldNewVal
}

// NewInMemoryChannelCache - construct new InMemoryChannelCache.
func NewInMemoryChannelCache() *InMemoryChannelCache {
	resp := &InMemoryChannelCache{
		chanCache:          make(map[string]string),
		chanMutex:          sync.Mutex{},
		deferredCacheMutex: sync.Mutex{},
		deferredCache:      make(map[string]OldNewVal),
	}

	return resp
}

// DeferredSet - set channel cache but do not commit it yet.
func (c *InMemoryChannelCache) DeferredSet(name, old, new string) {
	c.deferredCacheMutex.Lock()
	defer c.deferredCacheMutex.Unlock()

	glog.V(3).Infof("DeferredSet %s old %s new %s", name, old, new)

	c.deferredCache[name] = OldNewVal{OldValue: old, NewValue: new}
}

// DeferredCommit - commit channel cache from DeferredSet.
func (c *InMemoryChannelCache) DeferredCommit() bool {
	c.Lock()
	c.deferredCacheMutex.Lock()
	defer c.Unlock()
	defer c.deferredCacheMutex.Unlock()

	for k, v := range c.deferredCache {
		val, exists := c.Get(k)
		if !exists || val == v.OldValue {
			glog.V(3).Infof("Commit ok %s from %s to %s", k, val, v.NewValue)
			c.Set(k, v.NewValue)
		} else {
			glog.V(3).Infof("Commit %s not same value %s vs %s", k, val, v.OldValue)
		}
	}

	for k := range c.deferredCache {
		delete(c.deferredCache, k)
	}

	return true
}

// DeferredRevert - revert channel cache from DeferredSet.
func (c *InMemoryChannelCache) DeferredRevert() bool {
	c.Lock()
	c.deferredCacheMutex.Lock()
	defer c.Unlock()
	defer c.deferredCacheMutex.Unlock()

	glog.V(3).Infof("DeferredRevert")
	for k := range c.deferredCache {
		delete(c.deferredCache, k)
	}

	return true
}
