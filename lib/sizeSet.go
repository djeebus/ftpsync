package lib

func NewSizeSet() *SizeSet {
	var set SizeSet

	set.m = make(map[string]int64)

	return &set
}

type SizeSet struct {
	m map[string]int64
}

func (ss *SizeSet) Set(path string, size int64) {
	ss.m[path] = size
}

func (ss *SizeSet) Len() int {
	return len(ss.m)
}

func (ss *SizeSet) Has(path string) bool {
	_, ok := ss.m[path]
	return ok
}

func (ss *SizeSet) Get(path string) (int64, bool) {
	size, ok := ss.m[path]
	return size, ok
}

func (ss *SizeSet) ToSet() *Set {
	set := NewSet()
	for key := range ss.m {
		set.Set(key)
	}
	return set
}
