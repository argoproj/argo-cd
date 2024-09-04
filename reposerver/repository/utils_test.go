package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

func TestGetCommonRootPath(t *testing.T) {
	tests := []struct {
		annotation       string
		appPath          string
		repoPath         string
		expectedRootPath string
	}{
		{".", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"},
		{"../../overlays;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731"},
		{"/services;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"},
		{"../../;..;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/team/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"},
		// backward compatibility test
		{"", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731"},
		// appPath should be the lower calculated root path
		{"./manifests", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/team/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/team/helloworld"},
		// glob pattern
		{"/services/shared/*-secret.yaml", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"},
	}

	for _, tt := range tests {
		req := &apiclient.ManifestRequest{AnnotationManifestGeneratePaths: tt.annotation}
		rootPath := GetApplicationRootPath(req, tt.appPath, tt.repoPath)
		assert.Equal(t, tt.expectedRootPath, rootPath, "input and output should match")
	}
}
