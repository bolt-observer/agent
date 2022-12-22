package filter

type AllowAllFilter struct {
	Filter
}

func NewAllowAllFilter() (FilterInterface, error) {
	return &AllowAllFilter{}, nil
}

func (f *AllowAllFilter) AllowPubKey(id string) bool {
	return true
}

func (f *AllowAllFilter) AllowChanID(id uint64) bool {
	return true
}

func (f *AllowAllFilter) AllowSpecial(private bool) bool {
	return true
}

type UnitTestFilter struct {
	Filter
}

func NewUnitTestFilter() (FilterInterface, error) {
	f := &UnitTestFilter{
		Filter: Filter{
			chanIdWhitelist: make(map[uint64]struct{}),
			nodeIdWhitelist: make(map[string]struct{}),
		},
	}

	return f, nil
}

func (u *UnitTestFilter) AddAllowPubKey(id string) {
	u.nodeIdWhitelist[id] = struct{}{}
}

func (u *UnitTestFilter) AddAllowChanID(id uint64) {
	u.chanIdWhitelist[id] = struct{}{}
}

func (f *UnitTestFilter) ChangeOptions(options Options) {
	f.Options = options
}
