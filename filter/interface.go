package filter

type FilterInterface interface {
	AllowPubKey(id string) bool
	AllowChanId(id uint64) bool
}

type Filter struct {
	chanIdWhitelist map[uint64]struct{}
	nodeIdWhitelist map[string]struct{}

	// Here we could have blacklists too
}

func (f *Filter) AllowPubKey(id string) bool {
	_, ok := f.nodeIdWhitelist[id]

	return ok
}

func (f *Filter) AllowChanId(id uint64) bool {
	_, ok := f.chanIdWhitelist[id]

	return ok
}
