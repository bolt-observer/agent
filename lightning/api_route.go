package lightning

import (
	"container/heap"
	"context"
	"fmt"
	"regexp"
)

// An Item is something we manage in a priority queue.
type Item struct {
	route DeterminedRoute
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
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

func IsValidPubKey(pubKey string) bool {
	r := regexp.MustCompile("^0[23][a-fA-F0-9]{64}$")

	return r.MatchString(pubKey)
}

// getRoutesTemplate implements Yen's algorithm for finding alternative routes
func getRoutesTemplate(ctx context.Context, l LightingAPICalls, source string, destination string, exclusions []Exclusion, msats int64) (<-chan DeterminedRoute, error) {
	ch := make(chan DeterminedRoute)

	// copy
	exclusionCopy := exclusions[:]

	// k == 1
	initial, err := l.GetRoute(ctx, source, destination, exclusions, msats)
	if err != nil {
		close(ch)
		return nil, err
	}

	pq := make(PriorityQueue, 0)
	item := &Item{
		route: DeterminedRoute{},
	}
	heap.Push(&pq, item)

	/*
		for pq.Len() > 0 {
			item := heap.Pop(&pq).(*Item)
			fmt.Printf("%+v ", item.route)
		}
	*/

	previous := initial[:]

	fmt.Printf("%+v, %+v\n", exclusionCopy, previous)

	ch <- initial

	// A[k-1] is previous
	go func() {
		defer close(ch)

		for {
			// TODO

			// The spur node ranges from the first node to the next to last node in the previous k-shortest path.
			//for i from 0 to size(A[k − 1]) − 2:

			ch <- DeterminedRoute{}
			previous = DeterminedRoute{}
		}
	}()

	return ch, nil
}

func (l *LndGrpcLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, msats int64) (<-chan DeterminedRoute, error) {
	return getRoutesTemplate(ctx, l, source, destination, exclusions, msats)
}

func (l *LndRestLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, msats int64) (<-chan DeterminedRoute, error) {
	panic("not implemented")
}

func (l *ClnRawLightningAPI) GetRoutes(ctx context.Context, source string, destination string, exclusions []Exclusion, msats int64) (<-chan DeterminedRoute, error) {
	return getRoutesTemplate(ctx, l, source, destination, exclusions, msats)
}
