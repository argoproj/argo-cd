package ujson

import (
	"fmt"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`null`,
			`
0  null`,
		},
		{
			"null\n", // end with newline
			`
0  null`,
		},
		{
			`{}`,
			`
0  {
0  }`,
		},
		{
			`{"foo":""}`,
			`
0  {
1 "foo" ""
0  }`,
		},
		{
			`{"foo": ""}`, // Space
			`
0  {
1 "foo" ""
0  }`,
		},
		{
			`{"foo":"bar"}`,
			`
0  {
1 "foo" "bar"
0  }`,
		},
		{
			`{"foo":"bar","baz":""}`,
			`
0  {
1 "foo" "bar"
1 "baz" ""
0  }`,
		},
		{
			`{ "foo" : "bar" , "baz" : 2 }`, // Space
			`
0  {
1 "foo" "bar"
1 "baz" 2
0  }`,
		},
		{
			`{"foo":null}`,
			`
0  {
1 "foo" null
0  }`,
		},
		{
			`{"foo":123}`,
			`
0  {
1 "foo" 123
0  }`,
		},
		{
			`{"foo":-123}`,
			`
0  {
1 "foo" -123
0  }`,
		},
		{
			`{"foo":42.1}`,
			`
0  {
1 "foo" 42.1
0  }`,
		},
		{
			`{"foo":+0}`,
			`
0  {
1 "foo" +0
0  }`,
		},
		{
			`{"foo":"b\"ar"}`,
			`
0  {
1 "foo" "b\"ar"
0  }`,
		},
		{
			`{"😀":"🎶\""}`,
			`
0  {
1 "😀" "🎶\""
0  }`,
		},
		{
			`{"foo":{}}`,
			`
0  {
1 "foo" {
1  }
0  }`,
		},
		{
			`{"foo":{"bar":false,"baz":true,"quix":null}}`,
			`
0  {
1 "foo" {
2 "bar" false
2 "baz" true
2 "quix" null
1  }
0  }`,
		},
		{
			`{"1":{"1.1":{"1.1.1":"foo","1.1.2":"bar"},"1.2":{"1.2.1":"baz"}}}`,
			`
0  {
1 "1" {
2 "1.1" {
3 "1.1.1" "foo"
3 "1.1.2" "bar"
2  }
2 "1.2" {
3 "1.2.1" "baz"
2  }
1  }
0  }`,
		},
		{
			`[]`,
			`
0  [
0  ]`,
		},
		{
			`[null]`,
			`
0  [
1  null
0  ]`,
		},
		{
			`[0]`,
			`
0  [
1  0
0  ]`,
		},
		{
			`["foo"]`,
			`
0  [
1  "foo"
0  ]`,
		},
		{
			`["",""]`,
			`
0  [
1  ""
1  ""
0  ]`,
		},
		{
			`["foo","bar"]`,
			`
0  [
1  "foo"
1  "bar"
0  ]`,
		},
		{
			`[[]]`,
			`
0  [
1  [
1  ]
0  ]`,
		},
		{
			`[{},[]]`,
			`
0  [
1  {
1  }
1  [
1  ]
0  ]`,
		},
		{
			`{"foo":[]}`,
			`
0  {
1 "foo" [
1  ]
0  }`,
		},
		{
			`{"foo":[{"k":"v"}]}`,
			`
0  {
1 "foo" [
2  {
3 "k" "v"
2  }
1  ]
0  }`,
		},
		{
			`{"foo":[{"k1":"v1","k2":"v2"}]}`,
			`
0  {
1 "foo" [
2  {
3 "k1" "v1"
3 "k2" "v2"
2  }
1  ]
0  }`,
		},
		{
			`{"foo":[{"k1.1":"v1.1","k1.2":"v1.2"},{"k2.1":"v2.1"}],"bar":{}}`,
			`
0  {
1 "foo" [
2  {
3 "k1.1" "v1.1"
3 "k1.2" "v1.2"
2  }
2  {
3 "k2.1" "v2.1"
2  }
1  ]
1 "bar" {
1  }
0  }`,
		},
		{
			`{"1":[{"2":{"k1":"v1","k2":"v2"}}]}`,
			`
0  {
1 "1" [
2  {
3 "2" {
4 "k1" "v1"
4 "k2" "v2"
3  }
2  }
1  ]
0  }`,
		},
		{
			`{"1":[{"2":[{"k1":"v1","k2":"v2"},{"k3":"v3"}]}]}`,
			`
0  {
1 "1" [
2  {
3 "2" [
4  {
5 "k1" "v1"
5 "k2" "v2"
4  }
4  {
5 "k3" "v3"
4  }
3  ]
2  }
1  ]
0  }`,
		},
		{
			`{ "1" : [ { "2": [ { "k1" : "v1" , "k2" : "v2" } ,{"k3":"v3" } ] } ] }`,
			`
0  {
1 "1" [
2  {
3 "2" [
4  {
5 "k1" "v1"
5 "k2" "v2"
4  }
4  {
5 "k3" "v3"
4  }
3  ]
2  }
1  ]
0  }`,
		},
	}

	for _, tt := range tests {
		t.Run("Walk/"+tt.input, func(t *testing.T) {
			var b strings.Builder
			err := Walk([]byte(tt.input),
				func(st int, key, value []byte) WalkFuncRtnType {
					fmt.Fprintf(&b, "\n%v %s %s", st, key, value)
					return WalkRtnValDefault
				})
			if err != nil {
				t.Error(err)
			} else if b.String() != tt.expected {
				t.Errorf("\nExpect: `%v`\nOutput: `%v`\n", tt.expected, b.String())
			}
		})
	}

	for _, tt := range tests {
		t.Run("Reconstruct/"+tt.input, func(t *testing.T) {
			// Handle the special testcase ending with newline. This test
			// reconstructs output json and compare with the input. Because
			// reconstruct will not append the last newline, so we must trim it
			// before comparing.
			expected := tt.input
			if expected[len(expected)-1] == '\n' {
				expected = expected[:len(expected)-1]
			}
			expected = strings.Replace(expected, " ", "", -1)

			data, err := Reconstruct([]byte(tt.input))
			if err != nil {
				t.Error(err)
			} else if s := string(data); s != expected {
				t.Errorf("\nExpect: %v\nOutput: %v\n", expected, s)
			}
		})
	}
}

func TestWalk_panic(t *testing.T) {
	input := []byte(`{"foo":f`)
	err := Walk(input, func(level int, key, value []byte) WalkFuncRtnType {
		return WalkRtnValDefault
	})
	if err == nil {
		t.Errorf("Expect error")
	}
}
