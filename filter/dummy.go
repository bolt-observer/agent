package filter

// AllowAllFilter struct.
type AllowAllFilter struct {
	Filter
}

// NewAllowAllFilter - creates a filter that allow everything.
func NewAllowAllFilter() (FilteringInterface, error) {
	return &AllowAllFilter{}, nil
}

// AllowPubKey returns true if pubkey should be allowed.
func (f *AllowAllFilter) AllowPubKey(id string) bool {
	return true
}

// AllowChanID returns true if short channel ID should be allowed.
func (f *AllowAllFilter) AllowChanID(id uint64) bool {
	return true
}

// AllowSpecial checks against bitmask and can allow all private or public channels.
func (f *AllowAllFilter) AllowSpecial(private bool) bool {
	return true
}

// UnitTestFilter struct.
type UnitTestFilter struct {
	Filter
}

// NewUnitTestFilter - creates a filter suitable for unit tests.
func NewUnitTestFilter() (FilteringInterface, error) {
	f := &UnitTestFilter{
		Filter: Filter{
			chanIDWhitelist: make(map[uint64]struct{}),
			nodeIDWhitelist: make(map[string]struct{}),
		},
	}

	return f, nil
}

// AddAllowPubKey - add pubkey to allow list.
func (u *UnitTestFilter) AddAllowPubKey(id string) {
	u.nodeIDWhitelist[id] = struct{}{}
}

// AddAllowChanID - add channel id to allow list.
func (u *UnitTestFilter) AddAllowChanID(id uint64) {
	u.chanIDWhitelist[id] = struct{}{}
}

// ChangeOptions - change options of the filter.
func (u *UnitTestFilter) ChangeOptions(options Options) {
	u.Options = options
}
