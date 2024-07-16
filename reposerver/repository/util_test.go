package repository

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
)

func TestGetAppPaths(t *testing.T) {

	var tests = []struct {
		annotation       string
		appPath          string
		repoPath         string
		expectedAppPaths []string
	}{
		// without annotations
		{"", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"}},
		// relative
		{".", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"}},
		{"../../overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays"}},
		{"../../overlays;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"}},
		// absolute
		{"/overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays"}},
		{"/overlays;/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"}},
		// relative & absolute mix
		{"/overlays;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"}},
		{"/overlays;../../services", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", []string{"/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/overlays", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"}},
	}

	for _, tt := range tests {
		req := &apiclient.ManifestRequest{AnnotationManifestGeneratePaths: tt.annotation}
		appPaths := GetAppPaths(req, tt.appPath, tt.repoPath)
		assert.Equal(t, tt.expectedAppPaths, appPaths, "input and output should match")
	}
}

func TestGetCommonRootPath(t *testing.T) {

	var tests = []struct {
		annotation       string
		appPath          string
		repoPath         string
		expectedRootPath string
	}{
		{"", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld"},
		{"../../overlays;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731"},
		{"/services;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"},
		{"../../;..;.", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services/team/helloworld", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731", "/tmp/_argocd-repo/7a58c52a-0030-4fd9-8cc5-35b2d8b4e731/services"},
	}

	for _, tt := range tests {
		req := &apiclient.ManifestRequest{AnnotationManifestGeneratePaths: tt.annotation}
		rootPath := GetCommonRootPath(req, tt.appPath, tt.repoPath)
		assert.Equal(t, tt.expectedRootPath, rootPath, "input and output should match")
	}
}
