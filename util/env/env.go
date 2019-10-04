package env

import (
	"fmt"
	"strings"
)

type Vars []Var

// returns the environ
func (e Vars) Environ() []string {
	var out []string
	for _, i := range e {
		out = append(out, string(i))
	}
	return out
}

// does an operation similar to `envstubst` tool
// see https://linux.die.net/man/1/envsubst
func (e Vars) Envsubst() func(s string) string {
	return func(s string) string {
		for _, e := range e {
			s = strings.ReplaceAll(s, fmt.Sprintf("$%s", e.Key()), e.Value())
			s = strings.ReplaceAll(s, fmt.Sprintf("${%s}", e.Key()), e.Value())
		}
		return s
	}
}

type Var string

func (e Var) parts() []string {
	return strings.SplitN(string(e), "=", 2)
}

func (e Var) Key() string {
	return e.parts()[0]
}

func (e Var) Value() string {
	return e.parts()[1]
}
