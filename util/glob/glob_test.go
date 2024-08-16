package glob

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Match(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		result  bool
	}{
		{"Exact match", "hello", "hello", true},
		{"Non-match exact", "hello", "hell", false},
		{"Long glob match", "hello", "hell*", true},
		{"Short glob match", "hello", "h*", true},
		{"Glob non-match", "hello", "e*", false},
		{"Invalid pattern", "e[[a*", "e[[a*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Match(tt.pattern, tt.input)
			assert.Equal(t, tt.result, res)
		})
	}
}

func Test_MatchList(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		list         []string
		patternMatch string
		result       bool
	}{
		{"Exact name in list", "test", []string{"test"}, EXACT, true},
		{"Exact name not in list", "test", []string{"other"}, EXACT, false},
		{"Exact name not in list, multiple elements", "test", []string{"some", "other"}, EXACT, false},
		{"Exact name not in list, list empty", "test", []string{}, EXACT, false},
		{"Exact name not in list, empty element", "test", []string{""}, EXACT, false},
		{"Glob name in list, but exact wanted", "test", []string{"*"}, EXACT, false},
		{"Glob name in list with simple wildcard", "test", []string{"*"}, GLOB, true},
		{"Glob name in list without wildcard", "test", []string{"test"}, GLOB, true},
		{"Glob name in list, multiple elements", "test", []string{"other*", "te*"}, GLOB, true},
		{"match everything but specified word: fail", "disallowed", []string{"/^((?!disallowed).)*$/"}, REGEXP, false},
		{"match everything but specified word: pass", "allowed", []string{"/^((?!disallowed).)*$/"}, REGEXP, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := MatchStringInList(tt.list, tt.input, tt.patternMatch)
			assert.Equal(t, tt.result, res)
		})
	}
}
