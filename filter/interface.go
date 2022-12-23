package filter

// Options bitmask for filter
type Options uint8

const (
	// None - no options
	None Options = 1 << iota
	// AllowAllPrivate - allow all private channels
	AllowAllPrivate
	// AllowAllPublic - allow all public channels
	AllowAllPublic
)

// FilteringInterface is the interface for all filters
type FilteringInterface interface {
	// AllowPubKey returns true if pubkey should be allowed
	AllowPubKey(id string) bool
	// AllowChanID returns true if short channel ID should be allowed
	AllowChanID(id uint64) bool
	// AllowSpecial checks against bitmask and can allow all private or public channels
	AllowSpecial(private bool) bool
}

// Filter is the main type
type Filter struct {
	// Options for the filter
	Options         Options
	chanIDWhitelist map[uint64]struct{}
	nodeIDWhitelist map[string]struct{}
}

// TODO: yeah it would be easier if we had just Allow(chan) or sth

// AllowPubKey returns true if pubkey should be allowed
func (f *Filter) AllowPubKey(id string) bool {
	_, ok := f.nodeIDWhitelist[id]

	return ok
}

// AllowChanID returns true if short channel ID should be allowed
func (f *Filter) AllowChanID(id uint64) bool {
	_, ok := f.chanIDWhitelist[id]

	return ok
}

// AllowSpecial checks against bitmask and can allow all private or public channels
func (f *Filter) AllowSpecial(private bool) bool {
	if private {
		return f.Options&AllowAllPrivate == AllowAllPrivate
	}

	return f.Options&AllowAllPublic == AllowAllPublic
}
