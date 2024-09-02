package regex

import (
	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"
)

func Match(pattern, text string) bool {
	compiledRegex, err := regexp2.Compile(pattern, 0)
	if err != nil {
		log.Warnf("failed to compile pattern %s due to error %v", pattern, err)
		return false
	}
	regexMatch, err := compiledRegex.MatchString(text)
	if err != nil {
		log.Warnf("failed to match pattern %s due to error %v", pattern, err)
		return false
	}
	return regexMatch
}
