package image

import (
	"regexp"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
)

// MatchFuncAny matches any pattern, i.e. always returns true
func MatchFuncAny(tagName string, args interface{}) bool {
	return true
}

// MatchFuncNone matches no pattern, i.e. always returns false
func MatchFuncNone(tagName string, args interface{}) bool {
	return false
}

// MatchFuncRegexp matches the tagName against regexp pattern and returns the result
func MatchFuncRegexp(tagName string, args interface{}) bool {
	pattern, ok := args.(*regexp.Regexp)
	if !ok {
		log.Errorf("args is not a RegExp")
		return false
	}
	return pattern.Match([]byte(tagName))
}
