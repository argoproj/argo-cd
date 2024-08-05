package glob

import (
	"strings"

	"github.com/argoproj/argo-cd/v2/util/regex"
)

// MatchStringInList will return true if item is contained in list. If
// exactMatch is set to false, list may contain globs to be matched.
func MatchStringInList(list []string, item string, exactMatch bool) bool {
	for _, ll := range list {
		// If string is wrapped in "/", assume it is a regular expression.
		if !exactMatch && strings.HasPrefix(ll, "/") && strings.HasSuffix(ll, "/") {
			if regex.Match(ll[1:len(ll)-1], item) {
				return true
			}
		} else if item == ll || (!exactMatch && Match(ll, item)) {
			return true
		}
	}
	return false
}
