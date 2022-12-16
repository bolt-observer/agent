package nodedata

import (
	"sync"

	"github.com/golang/glog"
)

type ChannelCache interface {
	Lock()
	Unlock()
	Get(name string) (string, bool)
	Set(name string, value string)
	DeferredSet(name, old, new string)
	DeferredCommit() bool
	DeferredRevert() bool
}

func (c *InMemoryChannelCache) Lock() {
	c.chanMutex.Lock()
}
func (c *InMemoryChannelCache) Unlock() {
	c.chanMutex.Unlock()
}

func (c *InMemoryChannelCache) Get(name string) (string, bool) {
	a, b := c.chanCache[name]
	return a, b
}
func (c *InMemoryChannelCache) Set(name string, value string) {
	c.chanCache[name] = value
}

type OldNewVal struct {
	OldValue string
	NewValue string
}

type InMemoryChannelCache struct {
	chanMutex sync.Mutex
	chanCache map[string]string

	deferredCacheMutex sync.Mutex
	deferredCache      map[string]OldNewVal
}

func NewInMemoryChannelCache() *InMemoryChannelCache {
	resp := &InMemoryChannelCache{
		chanCache:          make(map[string]string),
		chanMutex:          sync.Mutex{},
		deferredCacheMutex: sync.Mutex{},
		deferredCache:      make(map[string]OldNewVal),
	}

	return resp
}

func (c *InMemoryChannelCache) DeferredSet(name, old, new string) {
	c.deferredCacheMutex.Lock()
	defer c.deferredCacheMutex.Unlock()

	glog.V(3).Infof("DeferredSet %s old %s new %s", name, old, new)

	c.deferredCache[name] = OldNewVal{OldValue: old, NewValue: new}
}

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
