package oci

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/pkg/sync"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func Test_nativeOCIClient_ResolveRevision(t *testing.T) {
	store := memory.New()
	descriptor := v1.Descriptor{
		MediaType: "",
		Digest:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	assert.NoError(t, store.Push(context.Background(), descriptor, bytes.NewReader([]byte(""))))
	assert.NoError(t, store.Tag(context.Background(), descriptor, "latest"))
	assert.NoError(t, store.Tag(context.Background(), descriptor, "1.2.0"))

	type fields struct {
		creds             Creds
		repoURL           string
		repo              oras.ReadOnlyTarget
		tagsFunc          func(context.Context, string) (tags []string, err error)
		repoLock          sync.KeyLock
		indexCache        indexCache
		repoCachePaths    io.TempPaths
		allowedMediaTypes []string
	}
	tests := []struct {
		name           string
		fields         fields
		revision       string
		noCache        bool
		expectedDigest string
		expectedError  error
	}{
		{
			name:     "resolve semantic version constraint",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve exact version",
			revision: "1.2.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve digest directly",
			revision: descriptor.Digest.String(),
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, fmt.Errorf("this should not be invoked")
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "no matching version for constraint",
			revision: "^3.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 4 tags"),
		},
		{
			name:     "error fetching tags",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, fmt.Errorf("some random error")
			}},
			expectedError: fmt.Errorf("error fetching tags: failed to get tags: some random error"),
		},
		{
			name:     "error resolving digest",
			revision: "sha256:abc123",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: fmt.Errorf("cannot get digest: not found"),
		},
		//new tests
		{
			name:     "resolve latest tag",
			revision: "latest",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "latest"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve with complex semver constraint",
			revision: ">=1.0.0 <2.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"0.9.0", "1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve with only non-semver tags",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"latest", "stable", "prod", "dev"}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 4 tags"),
		},
		{
			name:     "resolve with empty tag list",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 0 tags"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClientWithLock(tt.fields.repoURL, tt.fields.creds, tt.fields.repoLock, tt.fields.repo, tt.fields.tagsFunc, tt.fields.allowedMediaTypes)
			got, err := c.ResolveRevision(context.Background(), tt.revision, tt.noCache)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
				return
			}
			if got != tt.expectedDigest {
				t.Errorf("ResolveRevision() got = %v, expectedDigest %v", got, tt.expectedDigest)
			}
		})
	}
}
