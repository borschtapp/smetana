package utils

// MapKeys returns the keys of a set-like map as a slice, for building SQL IN clauses.
func MapKeys[K comparable](m map[K]bool) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
