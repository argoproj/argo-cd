package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/util"
	"github.com/argoproj/argo-cd/v2/util/webhook"
)

func TestMakeSignature(t *testing.T) {
	for size := 1; size <= 64; size++ {
		s, err := util.MakeSignature(size)
		if err != nil {
			t.Errorf("Could not generate signature of size %d: %v", size, err)
		}
		t.Logf("Generated token: %v", s)
	}
}

func TestParseRevision(t *testing.T) {
	type args struct {
		ref string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "gitflow", args: args{ref: "refs/heads/fix/appset-webhook"}, want: "fix/appset-webhook"},
		{name: "tag", args: args{ref: "refs/tags/v3.14.1"}, want: "v3.14.1"},
		{name: "env-branch", args: args{ref: "refs/heads/env/dev"}, want: "env/dev"},
		{name: "main", args: args{ref: "refs/heads/main"}, want: "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, webhook.ParseRevision(tt.args.ref), "parseRevision(%v)", tt.args.ref)
		})
	}
}
