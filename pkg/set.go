package pkg

func newSet() *Set {
	var set Set

	set.m = make(map[string]struct{})

	return &set
}

type Set struct {
	m map[string]struct{}
}

func (s *Set) Has(key string) bool {
	_, ok := s.m[key]
	return ok
}

func (s *Set) Set(key string) {
	s.m[key] = struct{}{}
}
