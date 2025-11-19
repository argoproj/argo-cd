package guard

import (
	"runtime/debug"
)

// Minimal logger contract; avoids depending on any specific logging package.
type Logger interface{ Errorf(string, ...any) }

// Run executes fn and recovers a panic, logging a component-specific message.
func RecoverAndLog(fn func(), log Logger, msg string) {
	defer func() {
		if r := recover(); r != nil {
			if log != nil {
				log.Errorf("%s: %v %s", msg, r, debug.Stack())
			}
		}
	}()
	fn()
}
