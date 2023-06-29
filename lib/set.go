package lib

func NewSet() *Set {
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

func (s *Set) Len() int {
	return len(s.m)
}

func (s *Set) Set(key string) {
	s.m[key] = struct{}{}
}

func (s *Set) ToList() []string {
	var list []string

	for key := range s.m {
		list = append(list, key)
	}

	return list
}

func (s *Set) Unset(key string) {
	delete(s.m, key)
}

func (s *Set) Union(other *Set) *Set {
	combined := NewSet()

	for k1 := range s.m {
		combined.Set(k1)
	}
	for k2 := range other.m {
		combined.Set(k2)
	}

	return combined
}

func (s *Set) Intersection(other *Set) *Set {
	result := NewSet()
	for k := range s.m {
		if other.Has(k) {
			result.Set(k)
		}
	}
	return result
}

func (s *Set) Difference(other *Set) *Set {
	diff := NewSet()
	for v1 := range s.m {
		diff.Set(v1)
	}
	for v2 := range other.m {
		if diff.Has(v2) {
			diff.Unset(v2)
		}
	}

	return diff
}
