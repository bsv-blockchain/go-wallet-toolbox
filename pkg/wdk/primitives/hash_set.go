package primitives

// HashSet represents a set of unique elements using a map
// with keys of any comparable type and boolean values.
type HashSet map[string]bool

// NewHashSet creates a new HashSet initialized with the provided values.
// Each value will be added as a key to the set.
func NewHashSet(vals ...string) HashSet {
	h := make(map[string]bool)
	for _, v := range vals {
		h[v] = true
	}
	return h
}

// Contains checks whether the specified key exists in the HashSet.
// It returns true if the key is present, and false otherwise.
func (h HashSet) Contains(k string) bool {
	_, ok := h[k]
	return ok
}
