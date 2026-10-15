package regex

import (
	"errors"
	"time"

	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"
)

// FilterMatchTimeout bounds how long one filter pattern may run against a single input.
const FilterMatchTimeout = time.Second

// CompileFilter compiles an untrusted filter pattern with RE2-compatible syntax, lookaround support and a bounded match time.
func CompileFilter(pattern string) (*regexp2.Regexp, error) {
	if hasLiteralQuoting(pattern) {
		return nil, errors.New(`\Q...\E literal quoting is not supported, escape special characters with a backslash instead`)
	}
	re, err := regexp2.Compile(pattern, regexp2.RE2)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = FilterMatchTimeout
	return re, nil
}

// hasLiteralQuoting reports whether pattern contains an unescaped \Q, which regexp2's RE2 mode compiles but never matches.
func hasLiteralQuoting(pattern string) bool {
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] != '\\' {
			continue
		}
		if pattern[i+1] == 'Q' {
			return true
		}
		i++
	}
	return false
}

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
