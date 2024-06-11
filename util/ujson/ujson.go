// Package ujson implements a fast and minimal JSON parser and transformer that
// works on unstructured json. Example use cases:
//
//  1. Walk through unstructured json:
//     - Print all keys and values
//     - Extract some values
//  2. Transform unstructured json:
//     - Remove all spaces
//     - Reformat
//     - Remove blacklist fields
//     - Wrap int64 in string for processing by JavaScript
//
// without fully unmarshalling it into a map[string]interface{}
//
// CAUTION: Behaviour is undefined on invalid json. Use on trusted input only.
//
// The single most important function is "Walk()", which parses the given json
// and call callback function for each key/value pair processed.
//
//	{
//	    "id": 12345,
//	    "name": "foo",
//	    "numbers": ["one", "two"],
//	    "tags": {"color": "red", "priority": "high"},
//	    "active": true
//	}
//
// Calling "Walk()" with the above input will produce:
//
//	| level | key        | value   |
//	|-------|------------|---------|
//	|   0   |            | {       |
//	|   1   | "id"       | 12345   |
//	|   1   | "name"     | "foo"   |
//	|   1   | "numbers"  | [       |
//	|   2   |            | "one"   |
//	|   2   |            | "two"   |
//	|   1   |            | ]       |
//	|   1   | "tags"     | {       |
//	|   2   | "color"    | "red"   |
//	|   2   | "priority" | "high"  |
//	|   1   |            | }       |
//	|   1   | "active"   | true    |
//	|   0   |            | }       |
//
// "level" indicates the indentation of the key/value pair as if the json is
// formatted properly. Keys and values are provided as raw literal. Strings are
// always double-quoted. To get the original string, use "Unquote".
//
// "value" will never be empty (for valid json). You can test the first byte
// ("value[0]") to get its type:
//
//   - 'n'     : Null ("null")
//   - 'f', 't': Boolean ("false", "true")
//   - '0'-'9' : Number
//   - '"'     : String, see "Unquote"
//   - '[', ']': Array
//   - '{', '}': Object
//
// When processing arrays and objects, first the open bracket ("[", "{") will be
// provided as "value", followed by its children, and finally the close bracket
// ("]", "}"). When encountering open brackets, you can make the callback
// function return "false" to skip the array/object entirely.
package ujson

import "fmt"

// return enum for WalkFunc
type WalkFuncRtnType int

const (
	WalkRtnValDefault    WalkFuncRtnType = iota
	WalkRtnValSkipObject                 // skip the current object or array
	WalkRtnValQuit                       // quit the walk
	WalkRtnValError                      // error
)

type WalkFunc func(level int, key, value []byte) WalkFuncRtnType

// Walk parses the given json and call "callback" for each key/value pair. See
// examples for sample callback params.
//
// The function "callback":
//
//   - may convert key and value to string for processing
//   - may return false to skip processing the current object or array
//   - must not modify any slice it receives.
func Walk(input []byte, callback WalkFunc) (ret error) {
	var key []byte
	i, si, ei, st, sst := 0, 0, 0, 0, 1024

	// return error, do not panic
	defer func() {
		if r := recover(); r != nil {
			ret = fmt.Errorf("µjson: error at %v: %v", i, r)
		}
	}()

	// trim the last newline
	if len(input) > 0 && input[len(input)-1] == '\n' {
		input = input[:len(input)-1]
	}

value:
	si = i
	switch input[i] {
	case 'n', 't': // null, true
		i += 4
		ei = i
		if st <= sst {
			switch callback(st, key, input[si:i]) {
			case WalkRtnValQuit:
				return nil
			}
		}
		key = nil
		goto closing
	case 'f': // false
		i += 5
		ei = i
		if st <= sst {
			switch callback(st, key, input[si:i]) {
			case WalkRtnValQuit:
				return nil
			}
		}
		key = nil
		goto closing
	case '{', '[':
		if st <= sst {
			switch callback(st, key, input[i:i+1]) {
			case WalkRtnValQuit:
				return nil
			case WalkRtnValSkipObject:
				sst = st
			}
		}
		key = nil
		st++
		i++
		for input[i] == ' ' ||
			input[i] == '\t' ||
			input[i] == '\n' ||
			input[i] == '\r' {
			i++
		}
		if input[i] == '}' || input[i] == ']' {
			goto closing
		}
		goto value
	case '"': // scan string
		for {
			i++
			switch input[i] {
			case '\\': // \. - skip 2
				i++
			case '"': // end of string
				i++
				ei = i // space, ignore
				for input[i] == ' ' ||
					input[i] == '\t' ||
					input[i] == '\n' ||
					input[i] == '\r' {
					i++
				}
				if input[i] != ':' {
					if st <= sst {
						switch callback(st, key, input[si:ei]) {
						case WalkRtnValQuit:
							return nil
						}
					}
					key = nil
				}
				goto closing
			}
		}
	case ' ', '\t', '\n', '\r': // space, ignore
		i++
		goto value
	default: // scan number
		for i < len(input) {
			switch input[i] {
			case ',', '}', ']', ' ', '\t', '\n', '\r':
				ei = i
				for input[i] == ' ' ||
					input[i] == '\t' ||
					input[i] == '\n' ||
					input[i] == '\r' {
					i++
				}
				if st <= sst {
					switch callback(st, key, input[si:ei]) {
					case WalkRtnValQuit:
						return nil
					}
				}
				key = nil
				goto closing
			}
			i++
		}
	}

closing:
	if i >= len(input) {
		return nil
	}
	switch input[i] {
	case ':':
		key = input[si:ei]
		i++
		goto value
	case ',':
		i++
		goto value
	case ']', '}':
		st--
		if st == sst {
			sst = 1024
		} else if st < sst {
			switch callback(st, nil, input[i:i+1]) {
			case WalkRtnValQuit:
				return nil
			}
		}
		if st <= 0 {
			return nil
		}
		i++
		goto closing
	case ' ', '\t', '\n', '\r':
		i++ // space, ignore
		goto closing
	default:
		return parseError(i, input[i], `expect ']', '}' or ','`)
	}
}

func parseError(i int, c byte, msg string) error {
	return fmt.Errorf("µjson: error at %v '%c' 0x%2x: %v", i, c, c, msg)
}

// ShouldAddComma decides if a comma should be appended while constructing
// output json. See Reconstruct for an example of rebuilding the json.
func ShouldAddComma(value []byte, lastChar byte) bool {
	// for valid json, the value will never be empty, so we can safely test only
	// the first byte
	return value[0] != '}' && value[0] != ']' &&
		lastChar != ',' && lastChar != '{' && lastChar != '['
}

// Reconstruct walks through the input json and rebuild it. It's put here as an
// example of using Walk.
func Reconstruct(input []byte) ([]byte, error) {
	b := make([]byte, 0, len(input))
	err := Walk(input, func(st int, key, value []byte) WalkFuncRtnType {
		if len(b) != 0 && ShouldAddComma(value, b[len(b)-1]) {
			b = append(b, ',')
		}
		if len(key) > 0 {
			b = append(b, key...)
			b = append(b, ':')
		}
		b = append(b, value...)
		return WalkRtnValDefault
	})
	return b, err
}
