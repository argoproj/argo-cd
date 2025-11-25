package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsGroupKindVisible(t *testing.T) {
	tests := []struct {
		name       string
		project    AppProject
		groupKind  schema.GroupKind
		namespaced bool
		want       bool
	}{
		{
			name: "permitted namespaced resource - visible",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "apps", Kind: "Deployment"},
			namespaced: true,
			want:       true,
		},
		{
			name: "not permitted but in blacklist with Visible=true - visible",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
					NamespaceResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "Secret", Visible: true},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "Secret"},
			namespaced: true,
			want:       true,
		},
		{
			name: "not permitted and not in blacklist - not visible",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "Secret"},
			namespaced: true,
			want:       false,
		},
		{
			name: "permitted cluster resource - visible",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "Namespace"},
			namespaced: false,
			want:       true,
		},
		{
			name: "cluster resource not permitted but in blacklist with Visible=true - visible",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
					ClusterResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "ClusterRole", Visible: true},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "ClusterRole"},
			namespaced: false,
			want:       true,
		},
		{
			name: "cluster resource not permitted and not visible in blacklist - not visible",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
					ClusterResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "ClusterRole", Visible: false},
					},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "ClusterRole"},
			namespaced: false,
			want:       false,
		},
		{
			name: "empty whitelist for namespaced - all permitted hence visible",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: nil,
				},
			},
			groupKind:  schema.GroupKind{Group: "apps", Kind: "Deployment"},
			namespaced: true,
			want:       true,
		},
		{
			name: "empty whitelist for cluster resource - not permitted, not visible",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{},
				},
			},
			groupKind:  schema.GroupKind{Group: "", Kind: "Namespace"},
			namespaced: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.project.IsGroupKindVisible(tt.groupKind, tt.namespaced)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsResourceActionable(t *testing.T) {
	tests := []struct {
		name        string
		project     AppProject
		groupKind   schema.GroupKind
		namespace   string
		destCluster *Cluster
		want        bool
		wantErr     bool
	}{
		{
			name: "permitted resource - actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			namespace:   "default",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        true,
			wantErr:     false,
		},
		{
			name: "not permitted but visible - actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
					NamespaceResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "Secret", Visible: true},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "", Kind: "Secret"},
			namespace:   "default",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        true,
			wantErr:     false,
		},
		{
			name: "not permitted and not visible - not actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					NamespaceResourceWhitelist: []metav1.GroupKind{
						{Group: "apps", Kind: "Deployment"},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "", Kind: "Secret"},
			namespace:   "default",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        false,
			wantErr:     false,
		},
		{
			name: "cluster resource permitted - actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "", Kind: "Namespace"},
			namespace:   "",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        true,
			wantErr:     false,
		},
		{
			name: "cluster resource not permitted but visible - actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
					ClusterResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "ClusterRole", Visible: true},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "", Kind: "ClusterRole"},
			namespace:   "",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        true,
			wantErr:     false,
		},
		{
			name: "cluster resource not permitted and not visible - not actionable",
			project: AppProject{
				Spec: AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{
						{Group: "", Kind: "Namespace"},
					},
					ClusterResourceBlacklist: []BlacklistEntry{
						{Group: "", Kind: "ClusterRole", Visible: false},
					},
				},
			},
			groupKind:   schema.GroupKind{Group: "", Kind: "ClusterRole"},
			namespace:   "",
			destCluster: &Cluster{Server: "https://kubernetes.default.svc"},
			want:        false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectClusters := func(_ string) ([]*Cluster, error) {
				clusters := make([]*Cluster, 0, len(tt.project.Spec.Destinations))
				for _, dest := range tt.project.Spec.Destinations {
					clusters = append(clusters, &Cluster{
						Server: dest.Server,
						Name:   dest.Name,
					})
				}
				return clusters, nil
			}

			got, err := tt.project.IsResourceActionable(tt.groupKind, tt.namespace, tt.destCluster, projectClusters)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
