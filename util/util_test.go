package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func TestSecretCopy(t *testing.T) {
	type args struct {
		secrets []*apiv1.Secret
	}
	tests := []struct {
		name string
		args args
		want []*apiv1.Secret
	}{
		{name: "nil", args: args{secrets: nil}, want: []*apiv1.Secret{}},
		{
			name: "Three", args: args{secrets: []*apiv1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "one"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "two"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "three"}},
			}},
			want: []*apiv1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "one"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "two"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "three"}},
			},
		},
		{
			name: "One", args: args{secrets: []*apiv1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "one"}}}},
			want: []*apiv1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "one"}}},
		},
		{name: "Zero", args: args{secrets: []*apiv1.Secret{}}, want: []*apiv1.Secret{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretsCopy := util.SecretCopy(tt.args.secrets)
			assert.Equalf(t, tt.want, secretsCopy, "SecretCopy(%v)", tt.args.secrets)
			for i := range tt.args.secrets {
				assert.NotSame(t, secretsCopy[i], tt.args.secrets[i])
			}
		})
	}
}
