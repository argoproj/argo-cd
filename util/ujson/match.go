package ujson

import (
	"strings"
)

type MatchCbFunc func(paths [][]byte, value []byte) error

type MatchCallback struct {
	paths []string
	cb    MatchCbFunc
}

type MatchOptions struct {
	IgnoreCase       bool
	QuitIfNoCallback bool
}

type MatchResult struct {
	Count int
}

const (
	InitialPathsDepth = 32
)

// iterate the json and call matching callback for each key/value pair.
// TODO: add support for object and array
func Match(input []byte, opt *MatchOptions, res *MatchResult, cbs ...*MatchCallback) error {
	paths := make([][]byte, 0, InitialPathsDepth)
	if opt == nil {
		opt = &MatchOptions{
			IgnoreCase:       true,
			QuitIfNoCallback: true,
		}
	}
	if res == nil {
		res = &MatchResult{
			Count: 0,
		}
	}
	err := Walk(input, func(level int, key, value []byte) WalkFuncRtnType {
		if len(cbs) == 0 {
			// do nothing if no callbacks
			// also skip the object or array
			if opt.QuitIfNoCallback {
				return WalkRtnValQuit
			}
		}
		// if level is equal to capacity of paths then double the size of paths
		if level >= cap(paths) {
			// double the size of paths
			newPaths := make([][]byte, len(paths), len(paths)*2)
			copy(newPaths, paths)
			paths = newPaths
		}
		if level == 0 {
			// if value to string is not { or } then return error
			if value[0] != '{' && value[0] != '}' {
				// skip the object or array
				return WalkRtnValSkipObject
			}
			return WalkRtnValDefault
		}

		newCbs := make([]*MatchCallback, 0, len(cbs))

		// set the key to the last element of paths
		pathIdx := level - 1
		paths = append(paths[:pathIdx], key)

		// increment count
		res.Count += 1

		// check all callbacks
		for _, cb := range cbs {
			if len(cb.paths) != 0 {
				// if length does not match then skip
				if len(cb.paths) != level {
					newCbs = append(newCbs, cb)
					continue
				}

				// if match, we call the callback but do not append to newCbs
				isMatch := true
				for i, p := range cb.paths {
					eq := false
					if opt.IgnoreCase {
						eq = strings.EqualFold(p, string(paths[i]))
					} else {
						eq = (p == string(paths[i]))
					}
					if !eq {
						isMatch = false
						break
					}

				}
				if !isMatch {
					newCbs = append(newCbs, cb)
					continue
				}
				if err := cb.cb(paths, value); err != nil {
					return WalkRtnValError
				}
			} else {
				// always append the match-all callback to newCbs
				newCbs = append(newCbs, cb)
				if err := cb.cb(paths, value); err != nil {
					return WalkRtnValError
				}
			}

		}
		cbs = newCbs
		return WalkRtnValDefault
	})
	return err
}
