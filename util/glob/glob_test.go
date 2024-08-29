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
		name   string
		input  string
		list   []string
		exact  bool
		result bool
	}{
		{"Exact name in list", "test", []string{"test"}, true, true},
		{"Exact name not in list", "test", []string{"other"}, true, false},
		{"Exact name not in list, multiple elements", "test", []string{"some", "other"}, true, false},
		{"Exact name not in list, list empty", "test", []string{}, true, false},
		{"Exact name not in list, empty element", "test", []string{""}, true, false},
		{"Glob name in list, but exact wanted", "test", []string{"*"}, true, false},
		{"Glob name in list with simple wildcard", "test", []string{"*"}, false, true},
		{"Glob name in list without wildcard", "test", []string{"test"}, false, true},
		{"Glob name in list, multiple elements", "test", []string{"other*", "te*"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := MatchStringInList(tt.list, tt.input, tt.exact)
			assert.Equal(t, tt.result, res)
		})
	}
}
