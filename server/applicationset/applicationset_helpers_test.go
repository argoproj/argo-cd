package applicationset

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestBuildApplicationSetTree(t *testing.T) {
	tests := []struct {
		name     string
		appset   *v1alpha1.ApplicationSet
		expected int // expected number of nodes in the tree
	}{
		{
			name: "empty applications",
			appset: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-appset",
					Namespace: "default",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{},
				},
			},
			expected: 0,
		},
		{
			name: "single application",
			appset: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-appset",
					Namespace: "default",
					UID:       "uid-1",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{
						{
							Name:    "app1",
							Group:   "argoproj.io",
							Version: "v1alpha1",
							Kind:    "Application",
							Health: &v1alpha1.HealthStatus{
								Status: health.HealthStatusHealthy,
							},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "multiple applications",
			appset: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-appset",
					Namespace: "default",
					UID:       "uid-2",
				},
				Status: v1alpha1.ApplicationSetStatus{
					Resources: []v1alpha1.ResourceStatus{
						{Name: "app1", Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"},
						{Name: "app2", Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"},
					},
				},
			},
			expected: 2,
		},
	}

	server := &Server{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := server.buildApplicationSetTree(tt.appset)
			assert.NoError(t, err)
			assert.Len(t, tree.Nodes, tt.expected)

			// Optional: check parent refs
			for _, node := range tree.Nodes {
				assert.Len(t, node.ParentRefs, 1)
				assert.Equal(t, tt.appset.Name, node.ParentRefs[0].Name)
			}
		})
	}
}

func TestValidateAppSet(t *testing.T) {
	tests := []struct {
		name              string
		appset            *v1alpha1.ApplicationSet
		expectedProject   string
		expectedErrString string
	}{
		{
			name:              "nil appset",
			appset:            nil,
			expectedProject:   "",
			expectedErrString: "ApplicationSet cannot be validated for nil value",
		},
		{
			name: "templated project",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "{{projectName}}",
						},
					},
				},
			},
			expectedProject:   "",
			expectedErrString: "the Argo CD API does not currently support creating ApplicationSets with templated `project` fields",
		},
		{
			name: "valid project",
			appset: &v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appset-valid",
					Namespace: "default",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "project-a",
						},
					},
				},
			},
			expectedProject:   "project-a",
			expectedErrString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{}
			project, err := s.validateAppSet(tt.appset)
			if tt.expectedErrString == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedProject, project)
			} else {
				assert.EqualError(t, err, tt.expectedErrString)
				assert.Equal(t, "", project)
			}
		})
	}
}

func TestServer_AppSetNamespaceOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		serverNS   string
		inputNS    string
		expectedNS string
	}{
		{
			name:       "empty namespace returns server default",
			serverNS:   "argocd",
			inputNS:    "",
			expectedNS: "argocd",
		},
		{
			name:       "provided namespace returned as-is",
			serverNS:   "argocd",
			inputNS:    "dev",
			expectedNS: "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Server{
				ns: tt.serverNS,
			}

			result := s.appsetNamespaceOrDefault(tt.inputNS)
			assert.Equal(t, tt.expectedNS, result)
		})
	}
}

func TestServer_IsNamespaceEnabled(t *testing.T) {
	tests := []struct {
		name              string
		enabledNamespaces []string
		namespaceToCheck  string
		expected          bool
	}{
		{
			name:              "namespace matches server namespace",
			enabledNamespaces: []string{},
			namespaceToCheck:  "argocd",
			expected:          true,
		},
		{
			name:              "namespace in enabled namespaces",
			enabledNamespaces: []string{"dev", "test"},
			namespaceToCheck:  "dev",
			expected:          true,
		},
		{
			name:              "namespace not in enabled namespaces",
			enabledNamespaces: []string{"dev", "test"},
			namespaceToCheck:  "prod",
			expected:          false,
		},
		{
			name:              "namespace matches glob pattern",
			enabledNamespaces: []string{"feature-*"},
			namespaceToCheck:  "feature-123",
			expected:          true,
		},
		{
			name:              "namespace matches regexp pattern",
			enabledNamespaces: []string{"/stage-[0-9]+/"},
			namespaceToCheck:  "stage-001",
			expected:          true,
		},
		{
			name:              "namespace does not match regexp",
			enabledNamespaces: []string{"/stage-[0-9]+/"},
			namespaceToCheck:  "staging",
			expected:          false,
		},
	}

	for _, tt := range tests {
		serverNs := "argocd"
		t.Run(tt.name, func(t *testing.T) {
			server := Server{
				ns:                serverNs,
				enabledNamespaces: tt.enabledNamespaces,
			}

			result := server.isNamespaceEnabled(tt.namespaceToCheck)
			assert.Equal(t, tt.expected, result)
		})
	}
}
