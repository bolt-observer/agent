package channelchecker

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	utils "github.com/bolt-observer/go_common/utils"
	"github.com/go-redis/redis"
	"github.com/golang/glog"
)

func (c *RedisChannelCache) Lock() {
	// noop
}
func (c *RedisChannelCache) Unlock() {
	// noop
}

func (c *RedisChannelCache) Get(name string) (string, bool) {
	id := fmt.Sprintf("%s_%s", c.env, name)
	exists := c.client.Exists(id).Val() > 0
	if !exists {
		return "", false
	}

	return c.client.Get(id).Val(), true
}

func (c *RedisChannelCache) Set(name string, value string) {
	id := fmt.Sprintf("%s_%s", c.env, name)
	c.client.Set(id, value, 0*time.Second)
}

type RedisChannelCache struct {
	client             *redis.Client
	env                string
	deferredCacheMutex sync.Mutex
	deferredCache      map[string]OldNewVal
}

func NewRedisChannelCache() *RedisChannelCache {
	db, err := strconv.Atoi(utils.GetEnvWithDefault("REDIS_DB", "1"))
	if err != nil {
		glog.Warningf("Error %+v\n", err)
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     utils.GetEnvWithDefault("REDIS_URL", "localhost:6379"),
		Password: utils.GetEnvWithDefault("REDIS_PASSWORD", ""),
		DB:       db,
	})

	resp := &RedisChannelCache{
		client:             client,
		env:                utils.GetEnvWithDefault("ENV", "develop"),
		deferredCacheMutex: sync.Mutex{},
		deferredCache:      make(map[string]OldNewVal),
	}

	return resp
}

func (c *RedisChannelCache) DeferredSet(name, old, new string) {
	c.deferredCacheMutex.Lock()
	defer c.deferredCacheMutex.Unlock()

	c.deferredCache[name] = OldNewVal{OldValue: old, NewValue: new}
}

func (c *RedisChannelCache) DeferredCommit() bool {
	c.Lock()
	c.deferredCacheMutex.Lock()
	defer c.Unlock()
	defer c.deferredCacheMutex.Unlock()

	for k, v := range c.deferredCache {
		val, exists := c.Get(k)
		if !exists || val == v.OldValue {
			c.Set(k, v.NewValue)
		}
	}

	for k := range c.deferredCache {
		delete(c.deferredCache, k)
	}

	return true
}

func (c *RedisChannelCache) DeferredRevert() bool {
	c.Lock()
	c.deferredCacheMutex.Lock()
	defer c.Unlock()
	defer c.deferredCacheMutex.Unlock()

	for k := range c.deferredCache {
		delete(c.deferredCache, k)
	}

	return true
}
