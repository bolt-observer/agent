package lightning

import (
	"container/heap"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

const (
	MaxPathLen = 20
)

// An Item is something we manage in a priority queue.
type Item struct {
	route DeterminedRoute
	index int
}

// Clone - create a copy of given DeterminedRoute
func (r DeterminedRoute) Clone() DeterminedRoute {
	ret := make(DeterminedRoute, len(r))
	copy(ret, r)
	return ret
}

// InitNameCache - get the mapping between pubkey and name
func InitNameCache(ctx context.Context, l LightingAPICalls) (map[string]string, error) {
	glog.Info("Initializing name cache... ")
	resp := make(map[string]string)
	graph, err := l.DescribeGraph(ctx, false)
	if err != nil {
		return resp, err
	}

	for _, one := range graph.Nodes {
		resp[one.PubKey] = one.Alias
	}
	glog.Info("Initializing name cache... done")

	return resp, nil
}

// PrettyRoute - returns a pretty route
func (r DeterminedRoute) PrettyRoute(destination string, chanIds bool, nameCache map[string]string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("(%d) ", len(r)))
	for _, one := range r {
		sb.WriteString(one.PubKey)
		if nameCache != nil {
			if name, ok := nameCache[one.PubKey]; ok {
				sb.WriteString(" (")
				sb.WriteString(name)
				sb.WriteString(")")
			}
		}
		if !chanIds {
			sb.WriteString(" --> ")
		} else {
			sb.WriteString(fmt.Sprintf(" --%d--> ", one.OutgoingChannelId))
		}
	}

	sb.WriteString(destination)

	return sb.String()
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

// Len - returns number of items in the PriorityQueue
func (pq PriorityQueue) Len() int { return len(pq) }

// Less - is element i less than element j
func (pq PriorityQueue) Less(i, j int) bool {
	return len(pq[i].route) > len(pq[j].route)
}

// Swap - swap elements i and j
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push - add ne element to queue
func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

// Pop - return last element from queue
func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) update(item *Item, value DeterminedRoute, priority int) {
	item.route = value
	heap.Fix(pq, item.index)
}

// Contains - check whether priority queue contains element
func (pq *PriorityQueue) Contains(value DeterminedRoute) bool {
	for _, one := range *pq {
		if routesMatch(one.route, value, len(value)) {
			return true
		}
	}

	return false
}

// IsValidPubKey - returns true if pubkey is valid, false otherwise
func IsValidPubKey(pubKey string) bool {
	r := regexp.MustCompile("^0[23][a-fA-F0-9]{64}$")

	return r.MatchString(pubKey)
}

func routesMatch(a DeterminedRoute, b DeterminedRoute, n int) bool {
	if n <= 0 {
		return true
	}

	for i := 0; i < n; i++ {
		if i >= len(a) || i >= len(b) {
			return false
		}
		if a[i].OutgoingChannelId != b[i].OutgoingChannelId || a[i].PubKey != b[i].PubKey {
			return false
		}
	}

	return true
}

// getRoutesTemplate implements Yen's algorithm for finding alternative routes
// Reference: https://en.wikipedia.org/wiki/Yen%27s_algorithm
func getRoutesTemplate(ctx context.Context, l LightingAPICalls, source string, destination string, exclusions []Exclusion, optimizeFor OptimizeRouteFor, msats int64) (<-chan DeterminedRoute, error) {
	// Beware each route will be saved so it's O(n) storage-wise!

	// This is A in the pseudo-code on Wikipedia
	oldRoutes := make([]DeterminedRoute, 0)
	// Output channel (iterator pattern)
	ch := make(chan DeterminedRoute, 1)
	getRoutesCalls := 0

	// k = 1
	initial, err := l.GetRoute(ctx, source, destination, exclusions, optimizeFor, msats)

	getRoutesCalls++
	glog.V(5).Infof("Number of get routes calls %d", getRoutesCalls)
	if err != nil {
		glog.Warningf("GetRoute returned error: %v", err)
		close(ch)
		return nil, err
	}

	if len(initial) > MaxPathLen {
		glog.Warningf("Too long path %d, returning error", len(initial))
		return nil, ErrRouteNotFound
	}

	// This is B in the pseudo-code on Wikipedia
	pq := make(PriorityQueue, 0)

	ch <- initial.Clone()
	oldRoutes = append(oldRoutes, initial)

	go func() {
		defer close(ch)

		// k = 2..inf
		for {
			if ctx.Err() != nil {
				return
			}

			// GetRoute is not deterministic, so perhaps we can just get away without Yen
			another, err := l.GetRoute(ctx, source, destination, exclusions, optimizeFor, msats)
			getRoutesCalls++
			if err != nil {
				glog.Warningf("GetRoute returned error: %v", err)
				continue
			}
			known := false
			for _, oldRoute := range oldRoutes {
				if routesMatch(oldRoute, another, len(another)) {
					known = true
					break
				}
			}
			if !known {
				ch <- another.Clone()
				oldRoutes = append(oldRoutes, another)
				continue
			}

			// Apparently not, try with Yen algorithm
			previousRoute := oldRoutes[len(oldRoutes)-1]
			eb := NewExclusionBuilder(exclusions)

			// The spur node ranges from the first node to the next to last node in the previous k-shortest path.
			// (Last node is omited in our format)
			for spurIndex := 0; spurIndex < len(previousRoute); spurIndex++ {
				rootPath := previousRoute[0:spurIndex] // without spur node

				// Ignore all nodes of root path
				for _, one := range rootPath {
					eb.AddNode(one.PubKey)
				}

				// Ignore old route outgoing connection if rootPath is same as an already known one
				for _, oldRoute := range oldRoutes {
					if routesMatch(oldRoute, rootPath, len(rootPath)) {
						eb.AddEdge(oldRoute[spurIndex].OutgoingChannelId)
					}
				}

				spurPath, err := l.GetRoute(ctx, previousRoute[spurIndex].PubKey, destination, eb.Build(), optimizeFor, msats)
				getRoutesCalls++
				glog.V(5).Infof("Number of get routes calls %d", getRoutesCalls)
				if err != nil {
					glog.Warningf("GetRoute returned error: %v", err)
					continue
				}

				totalPath := rootPath.Clone() // or else oldRoutes will be updated which is bad!
				totalPath = append(totalPath, spurPath...)
				if len(totalPath) > MaxPathLen {
					glog.Warningf("Too long path %d, skipping", len(totalPath))
					continue
				}

				// If not in B append to B
				if !pq.Contains(totalPath) {
					heap.Push(&pq, &Item{route: totalPath})
				}
			}

			// B is empty
			if pq.Len() == 0 {
				return
			}

			item := heap.Pop(&pq).(*Item)
			ch <- item.route.Clone()
			oldRoutes = append(oldRoutes, item.route)
		}
	}()

	return ch, nil
}

func (l *LndGrpcLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, optimizeFor OptimizeRouteFor, msats int64) (<-chan DeterminedRoute, error) {
	return getRoutesTemplate(ctx, l, source, destination, exclusions, optimizeFor, msats)
}

func (l *LndRestLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, optimizeFor OptimizeRouteFor, msats int64) (<-chan DeterminedRoute, error) {
	panic("not implemented")
}

func (l *ClnRawLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, optimizeFor OptimizeRouteFor, msats int64) (<-chan DeterminedRoute, error) {
	return getRoutesTemplate(ctx, l, source, destination, exclusions, optimizeFor, msats)
}
