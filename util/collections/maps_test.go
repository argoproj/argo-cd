package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyStringMap(t *testing.T) {
	out := CopyStringMap(map[string]string{"foo": "bar"})
	assert.Equal(t, map[string]string{"foo": "bar"}, out)
}

func TestStringMapsEqual(t *testing.T) {
	assert.True(t, StringMapsEqual(nil, nil))
	assert.True(t, StringMapsEqual(nil, map[string]string{}))
	assert.True(t, StringMapsEqual(map[string]string{}, nil))
	assert.True(t, StringMapsEqual(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"}))
	assert.False(t, StringMapsEqual(map[string]string{"foo": "bar"}, nil))
	assert.False(t, StringMapsEqual(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar1"}))
}

func TestMergeStringMaps(t *testing.T) {
	tests := []struct {
		name string
		args []map[string]string
		want map[string]string
	}{
		{
			name: "test single map",
			args: []map[string]string{
				{"foo": "bar"},
				{"foo1": "bar1"},
			},
			want: map[string]string{
				"foo":  "bar",
				"foo1": "bar1",
			},
		},
		{
			name: "test contains nil map",
			args: []map[string]string{
				{"foo": "bar"},
				nil,
				{"foo1": "bar1"},
			},
			want: map[string]string{
				"foo":  "bar",
				"foo1": "bar1",
			},
		},
		{
			name: "test contains multiple maps",
			args: []map[string]string{
				{"foo": "bar"},
				{
					"foo1": "bar1",
					"foo2": "bar2",
				},
				{
					"foo":  "bar1",
					"foo2": "bar2",
					"foo3": "bar3",
				},
			},
			want: map[string]string{
				"foo":  "bar1",
				"foo1": "bar1",
				"foo2": "bar2",
				"foo3": "bar3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, MergeStringMaps(tt.args...), "MergeStringMaps(%v)", tt.args)
		})
	}
}
