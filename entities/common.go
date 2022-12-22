package entities

import (
	"sync"

	api "github.com/bolt-observer/agent/lightningapi"
	"golang.org/x/sync/semaphore"
)

type NodeIdentifier struct {
	Identifier string `json:"identifier"`
	UniqueId   string `json:"unique_id"`
}

func (n *NodeIdentifier) GetId() string {
	return n.Identifier + n.UniqueId
}

type NewApiCall func() api.LightingApiCalls

type ReentrancyBlock struct {
	mutex     sync.Mutex
	semaphore map[string]*semaphore.Weighted
	num       int64
}

func NewReentrancyBlock() *ReentrancyBlock {
	return &ReentrancyBlock{semaphore: make(map[string]*semaphore.Weighted), num: 1}
}

func (r *ReentrancyBlock) Enter(id string) bool {
	var sem *semaphore.Weighted

	r.mutex.Lock()
	if _, ok := r.semaphore[id]; !ok {
		r.semaphore[id] = semaphore.NewWeighted(r.num)
	}

	sem = r.semaphore[id]
	r.mutex.Unlock()

	return sem.TryAcquire(1)
}

func (r *ReentrancyBlock) Release(id string) {
	var sem *semaphore.Weighted

	r.mutex.Lock()
	if _, ok := r.semaphore[id]; !ok {
		r.mutex.Unlock()
		return
	}
	sem = r.semaphore[id]
	r.mutex.Unlock()

	sem.Release(1)
}
