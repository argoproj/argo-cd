package glob

// MatchStringInList will return true if item is contained in list. If
// exactMatch is set to false, list may contain globs to be matched.
func MatchStringInList(list []string, item string, exactMatch bool) bool {
	for _, ll := range list {
		if item == ll || (!exactMatch && Match(ll, item)) {
			return true
		}
	}
	return false
}
