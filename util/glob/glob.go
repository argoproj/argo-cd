package glob

import (
	"github.com/gobwas/glob"
	log "github.com/sirupsen/logrus"
)

func Match(pattern, text string, seperators ...rune) bool {
	compiledGlob, err := glob.Compile(pattern, seperators...)
	if err != nil {
		log.Warnf("failed to compile pattern %s due to error %v", pattern, err)
		return false
	}
	return compiledGlob.Match(text)
}
