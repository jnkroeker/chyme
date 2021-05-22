package core

import "sort"

// Converts a map[string]string to an array of [key, value] tuples sorted by the keys.
func mapToSortedTuples(m map[string]string) [][2]string {
	n := len(m)
	keys := make([]string, n)
	tuples := make([][2]string, n)

	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	for i, k := range keys {
		tuples[i] = [2]string{k, m[k]}
	}
	return tuples
}