package filter

type AllowAllFilter struct {
	Filter
}

func NewAllowAllFilter() (FilteringInterface, error) {
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

func NewUnitTestFilter() (FilteringInterface, error) {
	f := &UnitTestFilter{
		Filter: Filter{
			chanIDWhitelist: make(map[uint64]struct{}),
			nodeIDWhitelist: make(map[string]struct{}),
		},
	}

	return f, nil
}

func (u *UnitTestFilter) AddAllowPubKey(id string) {
	u.nodeIDWhitelist[id] = struct{}{}
}

func (u *UnitTestFilter) AddAllowChanID(id uint64) {
	u.chanIDWhitelist[id] = struct{}{}
}

func (f *UnitTestFilter) ChangeOptions(options Options) {
	f.Options = options
}
