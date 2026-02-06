package collections

import "maps"

// Merge takes a collection of maps and returns a single map, where by items are merged of all given sets.
// If any keys overlap, Then the last specified key takes precedence.
// Example:
//
//	data := collections.Merge(map[string]string{"foo": "bar1", "baz": "bar1"}, map[string]string{"foo": "bar2"}) // returns: map[string]string{"foo": "bar2", "empty": "bar1"}
func Merge[K comparable, V any](items ...map[K]V) map[K]V {
	res := make(map[K]V)
	for _, m := range items {
		maps.Copy(res, m)
	}
	return res
}
