package text

import (
	"testing"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Trunc(tt.args.message, tt.args.n); got != tt.want {
				t.Errorf("Trunc() = %v, want %v", got, tt.want)
			}
		})
	}
}
