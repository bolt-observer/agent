package channelchecker

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	utils "github.com/bolt-observer/go_common/utils"
	"github.com/getsentry/sentry-go"
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
	err := c.client.Exists(id).Err()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Redis Get error %v\n", err)
		return "", false
	}

	exists := c.client.Exists(id).Val() > 0
	if !exists {
		return "", false
	}

	return c.client.Get(id).Val(), true
}

func (c *RedisChannelCache) Set(name string, value string) {
	id := fmt.Sprintf("%s_%s", c.env, name)
	status := c.client.Set(id, value, 0*time.Second)
	if status.Err() != nil {
		fmt.Fprintf(os.Stderr, "Redis Set error %v\n", status.Err())
	}
}

type RedisChannelCache struct {
	client             *redis.Client
	env                string
	deferredCacheMutex sync.Mutex
	deferredCache      map[string]OldNewVal
}

func removeQueryParams(in string) string {
	u, err := url.Parse(in)
	if err != nil {
		return in
	}
	u.RawQuery = ""
	return u.String()
}

func NewRedisChannelCache() *RedisChannelCache {
	url := removeQueryParams(utils.GetEnvWithDefault("REDIS_URL", "redis://127.0.0.1:6379/1"))

	opts, err := redis.ParseURL(url)
	if err != nil {
		sentry.CaptureException(err)
		glog.Warningf("Redis url is bad %s %v", url, err)
		return nil
	}

	client := redis.NewClient(opts)

	resp := &RedisChannelCache{
		client:             client,
		env:                utils.GetEnvWithDefault("ENV", "develop"),
		deferredCacheMutex: sync.Mutex{},
		deferredCache:      make(map[string]OldNewVal),
	}

	if client.Info().Err() != nil {
		sentry.CaptureMessage(fmt.Sprintf("Redis seems to be not usable %s", url))
		glog.Warningf("Redis seems to be not usable %s", url)
		return nil
	}

	return resp
}

func (c *RedisChannelCache) DeferredSet(name, old, new string) {
	c.deferredCacheMutex.Lock()
	defer c.deferredCacheMutex.Unlock()

	glog.V(3).Infof("DeferredSet %s old %s new %s", name, old, new)

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
