package filter

type Options uint8

const (
	None Options = 1 << iota
	AllowAllPrivate
	AllowAllPublic
)

type FilterInterface interface {
	AllowPubKey(id string) bool
	AllowChanId(id uint64) bool
	AllowSpecial(private bool) bool
}

type Filter struct {
	Options         Options
	chanIdWhitelist map[uint64]struct{}
	nodeIdWhitelist map[string]struct{}
}

// TODO: yeah it would be easier if we had just Allow(chan) or sth

func (f *Filter) AllowPubKey(id string) bool {
	_, ok := f.nodeIdWhitelist[id]

	return ok
}

func (f *Filter) AllowChanId(id uint64) bool {
	_, ok := f.chanIdWhitelist[id]

	return ok
}

func (f *Filter) AllowSpecial(private bool) bool {
	if private {
		return f.Options&AllowAllPrivate == AllowAllPrivate
	} else {
		return f.Options&AllowAllPublic == AllowAllPublic
	}
}
