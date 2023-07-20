package lightning

import (
	"container/heap"
	"context"
	"fmt"
	"regexp"
	"strings"
)

// An Item is something we manage in a priority queue.
type Item struct {
	route DeterminedRoute
	index int
}

// PrettyRoute - returns a pretty route
func (r DeterminedRoute) PrettyRoute(destination string, chanIds bool) string {
	var sb strings.Builder

	for _, one := range r {
		sb.WriteString(one.PubKey)
		if !chanIds {
			sb.WriteString(" --> ")
		} else {
			sb.WriteString(fmt.Sprintf(" --%d--> ", one.OutgoingChannelId))
		}
	}

	sb.WriteString(destination)
	sb.WriteString("\n")

	return sb.String()
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return len(pq[i].route) > len(pq[j].route)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

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

func (pq *PriorityQueue) Contains(value DeterminedRoute) bool {
	for _, one := range *pq {
		if routesMatch(one.route, value, len(one.route)) {
			return true
		}
	}

	return false
}

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
		if a[i].OutgoingChannelId != b[i].OutgoingChannelId || a[i].PubKey == b[i].PubKey {
			return false
		}
	}

	return true
}

// ExlusionBuilder struct.
type ExclusionBuilder struct {
	nodeExclusion map[string]struct{}
	edgeExclusion map[uint64]struct{}
}

// AddNode - excludes node by pubkey
func (b ExclusionBuilder) AddNode(node string) {
	b.nodeExclusion[node] = struct{}{}
}

// AddEdge - excluse edge by channel id
func (b ExclusionBuilder) AddEdge(edge uint64) {
	b.edgeExclusion[edge] = struct{}{}
}

// NewEmptyExclusionBuilder - creates a new empty builder
func NewEmptyExclusionBuilder() ExclusionBuilder {
	ret := ExclusionBuilder{}
	ret.edgeExclusion = make(map[uint64]struct{})
	ret.nodeExclusion = make(map[string]struct{})

	return ret
}

// NewExclusionBuilder - creates a new builder with existing exclusions
func NewExclusionBuilder(existing []Exclusion) ExclusionBuilder {
	ret := NewEmptyExclusionBuilder()

	for _, exclusion := range existing {
		switch e := exclusion.(type) {
		case ExcludedNode:
			ret.AddNode(e.PubKey)
		case ExcludedEdge:
			ret.AddEdge(e.ChannelId)
		}
	}

	return ret
}

func (b ExclusionBuilder) Build() []Exclusion {
	ret := make([]Exclusion, 0, len(b.nodeExclusion)+len(b.edgeExclusion))

	for k := range b.nodeExclusion {
		ret = append(ret, ExcludedNode{PubKey: k})
	}

	for k := range b.edgeExclusion {
		ret = append(ret, ExcludedEdge{ChannelId: k})
	}

	return ret
}

// getRoutesTemplate implements Yen's algorithm for finding alternative routes
// Reference: https://en.wikipedia.org/wiki/Yen%27s_algorithm
func getRoutesTemplate(ctx context.Context, l LightingAPICalls, source string, destination string, exclusions []Exclusion, optimizeFor OptimizeRouteFor, msats int64) (<-chan DeterminedRoute, error) {
	// Beware each route will be saved so it's O(n) storage-wise!

	// This is A in the algorithm
	oldRoutes := make([]DeterminedRoute, 0)
	// Output channel (iterator pattern)
	ch := make(chan DeterminedRoute)

	// k = 1
	initial, err := l.GetRoute(ctx, source, destination, exclusions, optimizeFor, msats)
	if err != nil {
		close(ch)
		return nil, err
	}

	// This is B in the algorithm
	pq := make(PriorityQueue, 0)

	ch <- initial
	oldRoutes = append(oldRoutes, initial)

	go func() {
		defer close(ch)

		// k = 2..inf
		for {
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

				// Ignore spureNode outgoing connection if rootPath is same an already known one
				for _, oldRoute := range oldRoutes {
					if routesMatch(oldRoute, rootPath, len(rootPath)) {
						eb.AddEdge(previousRoute[spurIndex].OutgoingChannelId)
						break
					}
				}

				spurPath, err := l.GetRoute(ctx, previousRoute[spurIndex].PubKey, destination, eb.Build(), optimizeFor, msats)
				if err != nil {
					continue
				}

				totalPath := append(rootPath, spurPath...)

				// If not in B append to B
				if !pq.Contains(totalPath) {
					heap.Push(&pq, Item{route: totalPath})
				}
			}

			// B is empty
			if pq.Len() == 0 {
				// empty
				return
			}

			item := heap.Pop(&pq).(*Item)
			ch <- item.route
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
