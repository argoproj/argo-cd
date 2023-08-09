package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombineStringMaps(t *testing.T) {
	testCases := []struct {
		name        string
		left        map[string]interface{}
		right       map[string]interface{}
		expected    map[string]string
		expectedErr error
	}{
		{
			name:        "combines the maps",
			left:        map[string]interface{}{"foo": "bar"},
			right:       map[string]interface{}{"a": "b"},
			expected:    map[string]string{"a": "b", "foo": "bar"},
			expectedErr: nil,
		},
		{
			name:        "fails if keys are the same but value isn't",
			left:        map[string]interface{}{"foo": "bar", "a": "fail"},
			right:       map[string]interface{}{"a": "b", "c": "d"},
			expected:    map[string]string{"a": "b", "foo": "bar"},
			expectedErr: fmt.Errorf("found duplicate key a with different value, a: fail ,b: b"),
		},
		{
			name:        "pass if keys & values are the same",
			left:        map[string]interface{}{"foo": "bar", "a": "b"},
			right:       map[string]interface{}{"a": "b", "c": "d"},
			expected:    map[string]string{"a": "b", "c": "d", "foo": "bar"},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			got, err := CombineStringMaps(testCaseCopy.left, testCaseCopy.right)

			if testCaseCopy.expectedErr != nil {
				assert.EqualError(t, err, testCaseCopy.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

		})
	}
}

func TestConvertToMapStringString(t *testing.T) {
	tests := []struct {
		name      string
		stringMap map[string]interface{}
		want      map[string]string
	}{
		{
			name: "normal test convert to map[string]string",
			stringMap: map[string]interface{}{
				"foo": "bar",
				"key": 123,
				"struct": struct {
					foo string
				}{
					foo: "bar",
				},
			},
			want: map[string]string{
				"foo":    "bar",
				"key":    "123",
				"struct": `{bar}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ConvertToMapStringString(tt.stringMap), "ConvertToMapStringString(%v)", tt.stringMap)
		})
	}
}

func TestCombineStringMapsAllowDuplicates(t *testing.T) {
	type args struct {
		aSI map[string]interface{}
		bSI map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "test combine string maps from left map",
			args: args{
				aSI: map[string]interface{}{
					"foo": "bar",
					"key": 123,
				},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "123",
			},
		},
		{
			name: "test combine string maps from right map",
			args: args{
				bSI: map[string]interface{}{
					"foo": "bar",
					"key": 123,
				},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "123",
			},
		},
		{
			name: "test combine string maps allow duplicates",
			args: args{
				aSI: map[string]interface{}{
					"foo": "bar",
					"key": 123,
				},
				bSI: map[string]interface{}{
					"foo":      "bar",
					"test-key": "test-value",
					"key2": struct {
						foo string
					}{
						foo: "bar",
					},
				},
			},
			want: map[string]string{
				"foo":      "bar",
				"key":      "123",
				"test-key": "test-value",
				"key2":     `{bar}`,
			},
		},
		{
			name: "test combine string maps allow duplicates with same key",
			args: args{
				aSI: map[string]interface{}{
					"foo":      "bar",
					"key":      123,
					"test-key": "test-val",
				},
				bSI: map[string]interface{}{
					"foo": "bar1",
					"key": 1234,
				},
			},
			want: map[string]string{
				"foo":      "bar1",
				"key":      "1234",
				"test-key": "test-val",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineStringMapsAllowDuplicates(tt.args.aSI, tt.args.bSI)
			assert.Equalf(t, tt.want, got, "CombineStringMapsAllowDuplicates(%v, %v)", tt.args.aSI, tt.args.bSI)
		})
	}
}

func TestConvertToMapStringInterface(t *testing.T) {
	tests := []struct {
		name      string
		stringMap map[string]string
		want      map[string]interface{}
	}{
		{
			name: "normal test convert to map[string]interface{}",
			stringMap: map[string]string{
				"foo": "bar",
				"key": "123",
			},
			want: map[string]interface{}{
				"foo": "bar",
				"key": "123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ConvertToMapStringInterface(tt.stringMap), "ConvertToMapStringInterface(%v)", tt.stringMap)
		})
	}
}
