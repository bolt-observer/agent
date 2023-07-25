package lightning

import "time"

// ExlusionBuilder struct.
type ExclusionBuilder struct {
	nodeExclusion map[string]time.Time
	edgeExclusion map[uint64]time.Time
}

// AddNode - excludes node by pubkey
func (b ExclusionBuilder) AddNode(node string) {
	MaxTime := time.Unix(1<<63-62135596801, 999999999)
	b.nodeExclusion[node] = MaxTime
}

// AddNodeWithDeadline - excludes node by pubkey
func (b ExclusionBuilder) AddNodeWithDeadline(node string, deadline time.Time) {
	b.nodeExclusion[node] = deadline
}

// AddEdgeWithDeadline - excluse edge by channel id
func (b ExclusionBuilder) AddEdgeWithDeadline(edge uint64, deadline time.Time) {
	b.edgeExclusion[edge] = deadline
}

// AddEdge - excluse edge by channel id
func (b ExclusionBuilder) AddEdge(edge uint64) {
	MaxTime := time.Unix(1<<63-62135596801, 999999999)
	b.edgeExclusion[edge] = MaxTime
}

// NewEmptyExclusionBuilder - creates a new empty builder
func NewEmptyExclusionBuilder() ExclusionBuilder {
	ret := ExclusionBuilder{}
	ret.edgeExclusion = make(map[uint64]time.Time)
	ret.nodeExclusion = make(map[string]time.Time)

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

// Build - get the built exclusions
func (b ExclusionBuilder) Build() []Exclusion {
	ret := make([]Exclusion, 0, len(b.nodeExclusion)+len(b.edgeExclusion))

	for k, v := range b.nodeExclusion {
		if v.Before(time.Now()) {
			continue
		}
		ret = append(ret, ExcludedNode{PubKey: k})
	}

	for k, v := range b.edgeExclusion {
		if v.Before(time.Now()) {
			continue
		}
		ret = append(ret, ExcludedEdge{ChannelId: k})
	}

	return ret
}
