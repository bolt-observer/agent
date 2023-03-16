package entities

import (
	"sync"

	api "github.com/bolt-observer/agent/lightning"
	"golang.org/x/sync/semaphore"
)

// NodeIdentifier represents the pubkey and optional unique identifier
type NodeIdentifier struct {
	Identifier string `json:"identifier"`
	UniqueID   string `json:"unique_id"`
}

// GetID obtains the string representation of the node identifier
func (n *NodeIdentifier) GetID() string {
	return n.Identifier + n.UniqueID
}

// NewAPICall is the signature of the function to get Lightning API
type NewAPICall func() (api.LightingAPICalls, error)

// ReentrancyBlock is used to block reentrancy based on string ID
type ReentrancyBlock struct {
	mutex     sync.Mutex
	semaphore map[string]*semaphore.Weighted
	num       int64
}

// NewReentrancyBlock constructs a new ReentrancyBlock
func NewReentrancyBlock() *ReentrancyBlock {
	return &ReentrancyBlock{semaphore: make(map[string]*semaphore.Weighted), num: 1}
}

// Enter is the acquire lock method (opposite of Release)
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

// Release is the drop lock method (opposite of Enter)
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

// Invalidatable interface.
type Invalidatable interface {
	Invalidate() error
}
