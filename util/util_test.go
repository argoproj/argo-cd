package util_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
		secrets []*corev1.Secret
	}
	tests := []struct {
		name string
		args args
		want []*corev1.Secret
	}{
		{name: "nil", args: args{secrets: nil}, want: []*corev1.Secret{}},
		{
			name: "Three", args: args{secrets: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "one"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "two"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "three"}},
			}},
			want: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "one"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "two"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "three"}},
			},
		},
		{
			name: "One", args: args{secrets: []*corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "one"}}}},
			want: []*corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "one"}}},
		},
		{name: "Zero", args: args{secrets: []*corev1.Secret{}}, want: []*corev1.Secret{}},
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

// TestGenerateCacheKey tests the GenerateCacheKey function
func TestGenerateCacheKey(t *testing.T) {
	// Define test cases
	testCases := []struct {
		format    string
		args      []any
		expected  string
		shouldErr bool
	}{
		{
			format:    "Hello %s",
			args:      []any{"World"},
			expected:  generateExpectedKey("Hello World"),
			shouldErr: false,
		},
		{
			format:    "",
			args:      []any{},
			expected:  generateExpectedKey(""),
			shouldErr: false,
		},
		{
			format:    "Number: %d",
			args:      []any{123},
			expected:  generateExpectedKey("Number: 123"),
			shouldErr: false,
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("format=%s args=%v", tc.format, tc.args), func(t *testing.T) {
			key, err := util.GenerateCacheKey(tc.format, tc.args...)
			if tc.shouldErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tc.expected {
				t.Fatalf("expected %s but got %s", tc.expected, key)
			}
		})
	}
}

// Helper function to generate the expected key
func generateExpectedKey(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
