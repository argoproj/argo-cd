package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrunc(t *testing.T) {
	type args struct {
		message string
		n       int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Empty", args{}, ""},
		{"5", args{message: "xxxxx", n: 5}, "xxxxx"},
		{"4", args{message: "xxxxx", n: 4}, "x..."},
		{"Multibyte", args{message: "こんにちは, world", n: 8}, "こんにちは..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Trunc(tt.args.message, tt.args.n); got != tt.want {
				t.Errorf("Trunc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemVer(t *testing.T) {
	assert.Equal(t, "1.4", SemVer("1.4"))
	assert.Equal(t, "1.4", SemVer("1.4+"))
}
