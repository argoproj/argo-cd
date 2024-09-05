package v1alpha1

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	argocdcommon "github.com/argoproj/argo-cd/v2/common"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestAppProject_IsSourcePermitted(t *testing.T) {
	testData := []struct {
		projSources []string
		appSource   string
		isPermitted bool
	}{{
		projSources: []string{"*"}, appSource: "https://github.com/argoproj/test.git", isPermitted: true,
	}, {
		projSources: []string{"https://github.com/argoproj/test.git"}, appSource: "https://github.com/argoproj/test.git", isPermitted: true,
	}, {
		projSources: []string{"ssh://git@GITHUB.com:argoproj/test"}, appSource: "ssh://git@github.com:argoproj/test", isPermitted: true,
	}, {
		projSources: []string{"https://github.com/argoproj/*"}, appSource: "https://github.com/argoproj/argoproj.git", isPermitted: true,
	}, {
		projSources: []string{"https://github.com/test1/test.git", "https://github.com/test2/test.git"}, appSource: "https://github.com/test2/test.git", isPermitted: true,
	}, {
		projSources: []string{"https://github.com/argoproj/test1.git"}, appSource: "https://github.com/argoproj/test2.git", isPermitted: false,
	}, {
		projSources: []string{"https://github.com/argoproj/*.git"}, appSource: "https://github.com/argoproj1/test2.git", isPermitted: false,
	}, {
		projSources: []string{"https://github.com/argoproj/foo"}, appSource: "https://github.com/argoproj/foo1", isPermitted: false,
	}, {
		projSources: []string{"https://gitlab.com/group/*"}, appSource: "https://gitlab.com/group/repo/owner", isPermitted: false,
	}, {
		projSources: []string{"https://gitlab.com/group/*/*"}, appSource: "https://gitlab.com/group/repo/owner", isPermitted: true,
	}, {
		projSources: []string{"https://gitlab.com/group/*/*/*"}, appSource: "https://gitlab.com/group/sub-group/repo/owner", isPermitted: true,
	}, {
		projSources: []string{"https://gitlab.com/group/**"}, appSource: "https://gitlab.com/group/sub-group/repo/owner", isPermitted: true,
	}}

	for _, data := range testData {
		proj := AppProject{
			Spec: AppProjectSpec{
				SourceRepos: data.projSources,
			},
		}
		assert.Equal(t, proj.IsSourcePermitted(ApplicationSource{
			RepoURL: data.appSource,
		}), data.isPermitted)
	}
}

func TestAppProject_IsNegatedSourcePermitted(t *testing.T) {
	testData := []struct {
		projSources []string
		appSource   string
		isPermitted bool
	}{{
		projSources: []string{"!https://github.com/argoproj/test.git"}, appSource: "https://github.com/argoproj/test.git", isPermitted: false,
	}, {
		projSources: []string{"!ssh://git@GITHUB.com:argoproj/test"}, appSource: "ssh://git@github.com:argoproj/test", isPermitted: false,
	}, {
		projSources: []string{"!https://github.com/argoproj/*"}, appSource: "https://github.com/argoproj/argoproj.git", isPermitted: false,
	}, {
		projSources: []string{"https://github.com/test1/test.git", "!https://github.com/test2/test.git"}, appSource: "https://github.com/test2/test.git", isPermitted: false,
	}, {
		projSources: []string{"!https://github.com/argoproj/foo*"}, appSource: "https://github.com/argoproj/foo1", isPermitted: false,
	}, {
		projSources: []string{"!https://gitlab.com/group/*/*"}, appSource: "https://gitlab.com/group/repo/owner", isPermitted: false,
	}, {
		projSources: []string{"!https://gitlab.com/group/*/*/*"}, appSource: "https://gitlab.com/group/sub-group/repo/owner", isPermitted: false,
	}, {
		projSources: []string{"!https://gitlab.com/group/**"}, appSource: "https://gitlab.com/group/sub-group/repo/owner", isPermitted: false,
	}, {
		projSources: []string{"*"}, appSource: "https://github.com/argoproj/test.git", isPermitted: true,
	}, {
		projSources: []string{"https://github.com/argoproj/test1.git", "*"}, appSource: "https://github.com/argoproj/test2.git", isPermitted: true,
	}, {
		projSources: []string{"!https://github.com/argoproj/*.git", "*"}, appSource: "https://github.com/argoproj1/test2.git", isPermitted: true,
	}}

	for _, data := range testData {
		proj := AppProject{
			Spec: AppProjectSpec{
				SourceRepos: data.projSources,
			},
		}
		assert.Equal(t, proj.IsSourcePermitted(ApplicationSource{
			RepoURL: data.appSource,
		}), data.isPermitted)
	}
}

func TestAppProject_IsDestinationPermitted(t *testing.T) {
	testData := []struct {
		projDest    []ApplicationDestination
		appDest     ApplicationDestination
		isPermitted bool
	}{
		{
			projDest: []ApplicationDestination{{
				Server: "https://kubernetes.default.svc", Namespace: "default",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://kubernetes.default.svc", Namespace: "default",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
			isPermitted: false,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://my-cluster", Namespace: "default",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
			isPermitted: false,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://kubernetes.default.svc", Namespace: "*",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://*.default.svc", Namespace: "default",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://team1-*", Namespace: "default",
			}},
			appDest:     ApplicationDestination{Server: "https://test2-dev-cluster", Namespace: "default"},
			isPermitted: false,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://kubernetes.default.svc", Namespace: "test-*",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "test-foo"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "https://kubernetes.default.svc", Namespace: "test-*",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "test"},
			isPermitted: false,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "*", Namespace: "*",
			}},
			appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "test"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "", Namespace: "*", Name: "test",
			}},
			appDest:     ApplicationDestination{Name: "test", Namespace: "test"},
			isPermitted: true,
		},
		{
			projDest: []ApplicationDestination{{
				Server: "", Namespace: "*", Name: "test2",
			}},
			appDest:     ApplicationDestination{Name: "test", Namespace: "test"},
			isPermitted: false,
		},
	}

	for _, data := range testData {
		proj := AppProject{
			Spec: AppProjectSpec{
				Destinations: data.projDest,
			},
		}
		permitted, _ := proj.IsDestinationPermitted(data.appDest, func(project string) ([]*Cluster, error) {
			return []*Cluster{}, nil
		})
		assert.Equal(t, data.isPermitted, permitted)
	}
}

func TestAppProject_IsNegatedDestinationPermitted(t *testing.T) {
	testData := []struct {
		projDest    []ApplicationDestination
		appDest     ApplicationDestination
		isPermitted bool
	}{{
		projDest: []ApplicationDestination{{
			Server: "!https://kubernetes.default.svc", Namespace: "default",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://kubernetes.default.svc", Namespace: "!default",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "!https://my-cluster", Namespace: "default",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "!https://kubernetes.default.svc", Namespace: "*",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "!https://*.default.svc", Namespace: "default",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "!https://team1-*", Namespace: "default",
		}},
		appDest:     ApplicationDestination{Server: "https://test2-dev-cluster", Namespace: "default"},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://kubernetes.default.svc", Namespace: "!test-*",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "test-foo"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://kubernetes.default.svc", Namespace: "!test-*",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "test"},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "", Namespace: "*", Name: "!test",
		}},
		appDest:     ApplicationDestination{Name: "test", Namespace: "test"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "", Namespace: "*", Name: "!test2",
		}},
		appDest:     ApplicationDestination{Name: "test", Namespace: "test"},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "kube-system",
		}, {
			Server: "*", Namespace: "!kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "*",
		}, {
			Server: "*", Namespace: "!kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://kubernetes.default.svc", Namespace: "*",
		}, {
			Server: "!https://kubernetes.default.svc", Namespace: "*",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "*",
		}, {
			Server: "!https://kubernetes.default.svc", Namespace: "kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "*",
		}, {
			Server: "!https://kubernetes.default.svc", Namespace: "kube-system",
		}, {
			Server: "*", Namespace: "!kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "*",
		}, {
			Server: "!https://kubernetes.default.svc", Namespace: "kube-system",
		}, {
			Server: "*", Namespace: "!kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "default"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "*", Namespace: "*",
		}, {
			Server: "!https://kubernetes.default.svc", Namespace: "kube-system",
		}, {
			Server: "*", Namespace: "!kube-system",
		}},
		appDest:     ApplicationDestination{Server: "https://test-dev-cluster", Namespace: "kube-system"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "", Namespace: "*", Name: "test",
		}, {
			Server: "", Namespace: "*", Name: "!test",
		}},
		appDest:     ApplicationDestination{Name: "test", Namespace: "test"},
		isPermitted: false,
	}}

	for _, data := range testData {
		proj := AppProject{
			Spec: AppProjectSpec{
				Destinations: data.projDest,
			},
		}
		permitted, _ := proj.IsDestinationPermitted(data.appDest, func(project string) ([]*Cluster, error) {
			return []*Cluster{}, nil
		})
		assert.Equal(t, data.isPermitted, permitted)
	}
}

func TestAppProject_IsDestinationPermitted_PermitOnlyProjectScopedClusters(t *testing.T) {
	testData := []struct {
		projDest    []ApplicationDestination
		appDest     ApplicationDestination
		clusters    []*Cluster
		isPermitted bool
	}{{
		projDest: []ApplicationDestination{{
			Server: "https://team1-*", Namespace: "default",
		}},
		clusters:    []*Cluster{},
		appDest:     ApplicationDestination{Server: "https://team1-something.com", Namespace: "default"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://my-cluster.123.com", Namespace: "default",
		}},
		appDest: ApplicationDestination{Server: "https://my-cluster.123.com", Namespace: "default"},
		clusters: []*Cluster{{
			Server: "https://my-cluster.123.com",
		}},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Server: "https://my-cluster.123.com", Namespace: "default",
		}},
		appDest: ApplicationDestination{Server: "https://some-other-cluster.example.com", Namespace: "default"},
		clusters: []*Cluster{{
			Server: "https://my-cluster.123.com",
		}},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Name: "some-server", Namespace: "default",
		}},
		clusters:    []*Cluster{},
		appDest:     ApplicationDestination{Name: "some-server", Namespace: "default"},
		isPermitted: false,
	}, {
		projDest: []ApplicationDestination{{
			Name: "some-other-server", Namespace: "default",
		}},
		appDest: ApplicationDestination{Name: "some-other-server", Namespace: "default"},
		clusters: []*Cluster{{
			Name: "some-other-server",
		}},
		isPermitted: true,
	}, {
		projDest: []ApplicationDestination{{
			Name: "some-server", Namespace: "default",
		}},
		appDest: ApplicationDestination{Name: "some-other-server", Namespace: "default"},
		clusters: []*Cluster{{
			Name: "some-server",
		}},
		isPermitted: false,
	}}

	for _, data := range testData {
		proj := AppProject{
			Spec: AppProjectSpec{
				PermitOnlyProjectScopedClusters: true,
				Destinations:                    data.projDest,
			},
		}

		permitted, _ := proj.IsDestinationPermitted(data.appDest, func(_ string) ([]*Cluster, error) {
			return data.clusters, nil
		})
		assert.Equal(t, data.isPermitted, permitted)
	}

	proj := AppProject{
		Spec: AppProjectSpec{
			PermitOnlyProjectScopedClusters: true,
			Destinations: []ApplicationDestination{{
				Server: "https://my-cluster.123.com", Namespace: "default",
			}},
		},
	}

	_, err := proj.IsDestinationPermitted(ApplicationDestination{Server: "https://my-cluster.123.com", Namespace: "default"}, func(_ string) ([]*Cluster, error) {
		return nil, errors.New("some error")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not retrieve project clusters")
}

func TestAppProject_IsGroupKindPermitted(t *testing.T) {
	proj := AppProject{
		Spec: AppProjectSpec{
			NamespaceResourceWhitelist: []metav1.GroupKind{},
			NamespaceResourceBlacklist: []metav1.GroupKind{{Group: "apps", Kind: "Deployment"}},
		},
	}
	assert.False(t, proj.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "ReplicaSet"}, true))
	assert.False(t, proj.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "Deployment"}, true))

	proj2 := AppProject{
		Spec: AppProjectSpec{
			NamespaceResourceWhitelist: []metav1.GroupKind{{Group: "apps", Kind: "ReplicaSet"}},
			NamespaceResourceBlacklist: []metav1.GroupKind{{Group: "apps", Kind: "Deployment"}},
		},
	}
	assert.True(t, proj2.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "ReplicaSet"}, true))
	assert.False(t, proj2.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "Action"}, true))

	proj3 := AppProject{
		Spec: AppProjectSpec{
			ClusterResourceBlacklist: []metav1.GroupKind{{Group: "", Kind: "Namespace"}},
		},
	}
	assert.False(t, proj3.IsGroupKindPermitted(schema.GroupKind{Group: "", Kind: "Namespace"}, false))

	proj4 := AppProject{
		Spec: AppProjectSpec{
			ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
			ClusterResourceBlacklist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
		},
	}
	assert.False(t, proj4.IsGroupKindPermitted(schema.GroupKind{Group: "", Kind: "Namespace"}, false))
	assert.True(t, proj4.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "Action"}, true))

	proj5 := AppProject{
		Spec: AppProjectSpec{
			ClusterResourceWhitelist:   []metav1.GroupKind{},
			NamespaceResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
		},
	}
	assert.False(t, proj5.IsGroupKindPermitted(schema.GroupKind{Group: "", Kind: "Namespace"}, false))
	assert.True(t, proj5.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "Action"}, true))

	proj6 := AppProject{
		Spec: AppProjectSpec{},
	}
	assert.False(t, proj6.IsGroupKindPermitted(schema.GroupKind{Group: "", Kind: "Namespace"}, false))
	assert.True(t, proj6.IsGroupKindPermitted(schema.GroupKind{Group: "apps", Kind: "Action"}, true))
}

func TestAppProject_GetRoleByName(t *testing.T) {
	t.Run("NotExists", func(t *testing.T) {
		p := &AppProject{}
		role, i, err := p.GetRoleByName("test-role")
		require.Error(t, err)
		assert.Equal(t, -1, i)
		assert.Nil(t, role)
	})
	t.Run("NotExists", func(t *testing.T) {
		p := AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role"}}}}
		role, i, err := p.GetRoleByName("test-role")
		require.NoError(t, err)
		assert.Equal(t, 0, i)
		assert.Equal(t, &ProjectRole{Name: "test-role"}, role)
	})
}

func TestAppProject_AddGroupToRole(t *testing.T) {
	t.Run("NoRole", func(t *testing.T) {
		p := &AppProject{}
		got, err := p.AddGroupToRole("test-role", "test-group")
		require.Error(t, err)
		assert.False(t, got)
	})
	t.Run("NoGroup", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{}}}}}
		got, err := p.AddGroupToRole("test-role", "test-group")
		require.NoError(t, err)
		assert.True(t, got)
		assert.Len(t, p.Spec.Roles[0].Groups, 1)
	})
	t.Run("Exists", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{"test-group"}}}}}
		got, err := p.AddGroupToRole("test-role", "test-group")
		require.NoError(t, err)
		assert.False(t, got)
	})
}

func TestAppProject_RemoveGroupFromRole(t *testing.T) {
	t.Run("NoRole", func(t *testing.T) {
		p := &AppProject{}
		got, err := p.RemoveGroupFromRole("test-role", "test-group")
		require.Error(t, err)
		assert.False(t, got)
	})
	t.Run("NoGroup", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{}}}}}
		got, err := p.RemoveGroupFromRole("test-role", "test-group")
		require.NoError(t, err)
		assert.False(t, got)
	})
	t.Run("Exists", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{"test-group"}}}}}
		got, err := p.RemoveGroupFromRole("test-role", "test-group")
		require.NoError(t, err)
		assert.True(t, got)
		assert.Empty(t, p.Spec.Roles[0].Groups)
	})
}

func newTestProject() *AppProject {
	p := AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj"},
		Spec:       AppProjectSpec{Roles: []ProjectRole{{Name: "my-role"}}, Destinations: []ApplicationDestination{{}}},
	}
	return &p
}

// TestAppProject_ValidateSources tests for an invalid source
func TestAppProject_ValidateSources(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	badSources := []string{
		"!*",
	}
	for _, badName := range badSources {
		p.Spec.SourceRepos = []string{badName}
		err = p.ValidateProject()
		require.Error(t, err)
	}

	duplicateSources := []string{
		"foo",
		"foo",
	}
	p.Spec.SourceRepos = duplicateSources
	err = p.ValidateProject()
	require.Error(t, err)
}

// TestAppProject_ValidateDestinations tests for an invalid destination
func TestAppProject_ValidateDestinations(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	badNamespaces := []string{
		"!*",
	}
	for _, badName := range badNamespaces {
		p.Spec.Destinations[0].Namespace = badName
		err = p.ValidateProject()
		require.Error(t, err)
	}

	goodNamespaces := []string{
		"*",
		"some-namespace",
	}
	for _, goodNamespace := range goodNamespaces {
		p.Spec.Destinations[0].Namespace = goodNamespace
		err = p.ValidateProject()
		require.NoError(t, err)
	}

	badServers := []string{
		"!*",
	}
	for _, badServer := range badServers {
		p.Spec.Destinations[0].Server = badServer
		err = p.ValidateProject()
		require.Error(t, err)
	}

	goodServers := []string{
		"*",
		"some-server",
	}
	for _, badName := range goodServers {
		p.Spec.Destinations[0].Server = badName
		err = p.ValidateProject()
		require.NoError(t, err)
	}

	badNames := []string{
		"!*",
	}
	for _, badName := range badNames {
		p.Spec.Destinations[0].Name = badName
		err = p.ValidateProject()
		require.Error(t, err)
	}

	goodNames := []string{
		"*",
		"some-name",
	}
	for _, goodName := range goodNames {
		p.Spec.Destinations[0].Name = goodName
		err = p.ValidateProject()
		require.NoError(t, err)
	}

	validDestination := ApplicationDestination{
		Server:    "some-server",
		Namespace: "some-namespace",
	}

	p.Spec.Destinations[0] = validDestination
	err = p.ValidateProject()
	require.NoError(t, err)

	// no duplicates allowed
	p.Spec.Destinations = []ApplicationDestination{validDestination, validDestination}
	err = p.ValidateProject()
	require.Error(t, err)

	cluster1Destination := ApplicationDestination{
		Name:      "cluster1",
		Namespace: "some-namespace",
	}
	cluster2Destination := ApplicationDestination{
		Name:      "cluster2",
		Namespace: "some-namespace",
	}
	// allow multiple destinations with blank server, same namespace but unique cluster name
	p.Spec.Destinations = []ApplicationDestination{cluster1Destination, cluster2Destination}
	err = p.ValidateProject()
	require.NoError(t, err)

	t.Run("must reject duplicate source namespaces", func(t *testing.T) {
		p.Spec.SourceNamespaces = []string{"argocd", "argocd"}
		err = p.ValidateProject()
		require.Error(t, err)
	})
}

// TestValidateRoleName tests for an invalid role name
func TestAppProject_ValidateRoleName(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	badRoleNames := []string{
		"",
		" ",
		"my role",
		"my, role",
		"my,role",
		"my\nrole",
		"my\rrole",
		"my:role",
		"my-role-",
		"-my-role",
	}
	for _, badName := range badRoleNames {
		p.Spec.Roles[0].Name = badName
		err = p.ValidateProject()
		require.Error(t, err)
	}
	goodRoleNames := []string{
		"MY-ROLE",
		"1MY-ROLE1",
	}
	for _, goodName := range goodRoleNames {
		p.Spec.Roles[0].Name = goodName
		err = p.ValidateProject()
		require.NoError(t, err)
	}
}

// TestValidateGroupName tests for an invalid group name
func TestAppProject_ValidateGroupName(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	p.Spec.Roles[0].Groups = []string{"mygroup"}
	err = p.ValidateProject()
	require.NoError(t, err)
	badGroupNames := []string{
		"",
		" ",
		"my, group",
		"my,group",
		"my\ngroup",
		"my\rgroup",
		" my:group",
		"my:group ",
	}
	for _, badName := range badGroupNames {
		p.Spec.Roles[0].Groups = []string{badName}
		err = p.ValidateProject()
		require.Error(t, err)
	}
	goodGroupNames := []string{
		"my:group",
	}
	for _, goodName := range goodGroupNames {
		p.Spec.Roles[0].Groups = []string{goodName}
		err = p.ValidateProject()
		require.NoError(t, err)
	}
}

func TestAppProject_ValidateSyncWindowList(t *testing.T) {
	t.Run("WorkingSyncWindow", func(t *testing.T) {
		p := newTestProjectWithSyncWindows()
		err := p.ValidateProject()
		require.NoError(t, err)
	})
	t.Run("HasNilSyncWindow", func(t *testing.T) {
		p := newTestProjectWithSyncWindows()
		err := p.ValidateProject()
		require.NoError(t, err)
		p.Spec.SyncWindows = append(p.Spec.SyncWindows, nil)
		err = p.ValidateProject()
		require.NoError(t, err)
	})
}

// TestInvalidPolicyRules checks various errors in policy rules
func TestAppProject_InvalidPolicyRules(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	type badPolicy struct {
		policy string
		errmsg string
	}
	badPolicies := []badPolicy{
		// incorrect form
		{"g, proj:my-proj:my-role, applications, get, my-proj/*, allow", "must be of the form: 'p, sub, res, act, obj, eft'"},
		{"p, not, enough, parts", "must be of the form: 'p, sub, res, act, obj, eft'"},
		{"p, has, too, many, parts, to, split", "must be of the form: 'p, sub, res, act, obj, eft'"},
		// invalid subject
		{"p, , applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:my-proj, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:my-proj:, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, ::, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:different-my-proj:my-role, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		// invalid resource
		{"p, proj:my-proj:my-role, , get, my-proj/*, allow", "resource must be: 'applications'"},
		{"p, proj:my-proj:my-role, applicationz, get, my-proj/*, allow", "resource must be: 'applications'"},
		{"p, proj:my-proj:my-role, projects, get, my-proj, allow", "resource must be: 'applications'"},
		// invalid action
		{"p, proj:my-proj:my-role, applications, , my-proj/*, allow", "invalid action"},
		{"p, proj:my-proj:my-role, applications, foo, my-proj/*, allow", "invalid action"},
		// invalid object
		{"p, proj:my-proj:my-role, applications, get, my-proj/, allow", "object must be of form"},
		{"p, proj:my-proj:my-role, applications, get, /, allow", "object must be of form"},
		{"p, proj:my-proj:my-role, applications, get, different-my-proj/*, allow", "object must be of form"},
		// invalid effect
		{"p, proj:my-proj:my-role, applications, get, my-proj/*, ", "effect must be: 'allow' or 'deny'"},
		{"p, proj:my-proj:my-role, applications, get, my-proj/*, foo", "effect must be: 'allow' or 'deny'"},
	}
	for _, bad := range badPolicies {
		p.Spec.Roles[0].Policies = []string{bad.policy}
		err = p.ValidateProject()
		require.Error(t, err)
		assert.Contains(t, err.Error(), bad.errmsg)
	}
}

// TestValidPolicyRules checks valid policy rules
func TestAppProject_ValidPolicyRules(t *testing.T) {
	p := newTestProject()
	err := p.ValidateProject()
	require.NoError(t, err)
	goodPolicies := []string{
		"p,proj:my-proj:my-role,applications,get,my-proj/*,allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*, deny",
		"p, proj:my-proj:my-role, applications, get, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*-foo, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/foo-*, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*-*, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*.*, allow",
		"p, proj:my-proj:my-role, applications, *, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, create, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, update, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, update/*, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, update/*/Pod/*, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, sync, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, delete, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, delete/*, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, delete/*/Pod/*, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, action/*, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, action/apps/Deployment/restart, my-proj/foo, allow",
	}
	for _, good := range goodPolicies {
		p.Spec.Roles[0].Policies = []string{good}
		err = p.ValidateProject()
		require.NoError(t, err)
	}
}

func TestExplicitType(t *testing.T) {
	src := ApplicationSource{
		Kustomize: &ApplicationSourceKustomize{
			NamePrefix: "foo",
		},
		Helm: &ApplicationSourceHelm{
			ValueFiles: []string{"foo"},
		},
	}
	explicitType, err := src.ExplicitType()
	require.Error(t, err)
	assert.Nil(t, explicitType)
	src = ApplicationSource{
		Helm: &ApplicationSourceHelm{
			ValueFiles: []string{"foo"},
		},
	}

	explicitType, err = src.ExplicitType()
	require.NoError(t, err)
	assert.Equal(t, ApplicationSourceTypeHelm, *explicitType)
}

func TestExplicitTypeWithDirectory(t *testing.T) {
	src := ApplicationSource{
		Helm:      &ApplicationSourceHelm{},
		Directory: &ApplicationSourceDirectory{},
	}
	_, err := src.ExplicitType()
	require.Error(t, err, "cannot add directory with any other types")
}

func TestAppSourceEquality(t *testing.T) {
	left := &ApplicationSource{
		Directory: &ApplicationSourceDirectory{
			Recurse: true,
		},
	}
	right := left.DeepCopy()
	assert.True(t, left.Equals(right))
	right.Directory.Recurse = false
	assert.False(t, left.Equals(right))
}

func TestAppSource_GetKubeVersionOrDefault(t *testing.T) {
	defaultKV := "999.999.999"
	cases := []struct {
		name   string
		source *ApplicationSource
		expect string
	}{
		{
			"nil source returns default",
			nil,
			defaultKV,
		},
		{
			"source without Helm or Kustomize returns default",
			&ApplicationSource{},
			defaultKV,
		},
		{
			"source with empty Helm returns default",
			&ApplicationSource{Helm: &ApplicationSourceHelm{}},
			defaultKV,
		},
		{
			"source with empty Kustomize returns default",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{}},
			defaultKV,
		},
		{
			"source with Helm override returns override",
			&ApplicationSource{Helm: &ApplicationSourceHelm{KubeVersion: "1.2.3"}},
			"1.2.3",
		},
		{
			"source with Kustomize override returns override",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{KubeVersion: "1.2.3"}},
			"1.2.3",
		},
	}

	for _, tc := range cases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			kv := tcc.source.GetKubeVersionOrDefault(defaultKV)
			assert.Equal(t, tcc.expect, kv)
		})
	}
}

func TestAppSource_GetAPIVersionsOrDefault(t *testing.T) {
	defaultAPIVersions := []string{"v1", "v2"}
	cases := []struct {
		name   string
		source *ApplicationSource
		expect []string
	}{
		{
			"nil source returns default",
			nil,
			defaultAPIVersions,
		},
		{
			"source without Helm or Kustomize returns default",
			&ApplicationSource{},
			defaultAPIVersions,
		},
		{
			"source with empty Helm returns default",
			&ApplicationSource{Helm: &ApplicationSourceHelm{}},
			defaultAPIVersions,
		},
		{
			"source with empty Kustomize returns default",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{}},
			defaultAPIVersions,
		},
		{
			"source with Helm override returns override",
			&ApplicationSource{Helm: &ApplicationSourceHelm{APIVersions: []string{"v3", "v4"}}},
			[]string{"v3", "v4"},
		},
		{
			"source with Kustomize override returns override",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{APIVersions: []string{"v3", "v4"}}},
			[]string{"v3", "v4"},
		},
	}

	for _, tc := range cases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			kv := tcc.source.GetAPIVersionsOrDefault(defaultAPIVersions)
			assert.Equal(t, tcc.expect, kv)
		})
	}
}

func TestAppSource_GetNamespaceOrDefault(t *testing.T) {
	defaultNS := "default"
	cases := []struct {
		name   string
		source *ApplicationSource
		expect string
	}{
		{
			"nil source returns default",
			nil,
			defaultNS,
		},
		{
			"source without Helm or Kustomize returns default",
			&ApplicationSource{},
			defaultNS,
		},
		{
			"source with empty Helm returns default",
			&ApplicationSource{Helm: &ApplicationSourceHelm{}},
			defaultNS,
		},
		{
			"source with empty Kustomize returns default",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{}},
			defaultNS,
		},
		{
			"source with Helm override returns override",
			&ApplicationSource{Helm: &ApplicationSourceHelm{Namespace: "not-default"}},
			"not-default",
		},
		{
			"source with Kustomize override returns override",
			&ApplicationSource{Kustomize: &ApplicationSourceKustomize{Namespace: "not-default"}},
			"not-default",
		},
	}

	for _, tc := range cases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			kv := tcc.source.GetNamespaceOrDefault(defaultNS)
			assert.Equal(t, tcc.expect, kv)
		})
	}
}

func TestAppDestinationEquality(t *testing.T) {
	left := &ApplicationDestination{
		Server:    "https://kubernetes.default.svc",
		Namespace: "default",
	}
	right := left.DeepCopy()
	assert.True(t, left.Equals(*right))
	right.Namespace = "kube-system"
	assert.False(t, left.Equals(*right))
}

func TestAppDestinationEquality_InferredServerURL(t *testing.T) {
	left := ApplicationDestination{
		Name:      "in-cluster",
		Namespace: "default",
	}
	right := ApplicationDestination{
		Name:             "in-cluster",
		Server:           "https://kubernetes.default.svc",
		Namespace:        "default",
		isServerInferred: true,
	}
	assert.True(t, left.Equals(right))
	assert.True(t, right.Equals(left))
}

func TestAppProjectSpec_DestinationClusters(t *testing.T) {
	tests := []struct {
		name         string
		destinations []ApplicationDestination
		want         []string
	}{
		{
			name:         "Empty",
			destinations: []ApplicationDestination{},
			want:         []string{},
		},
		{
			name:         "SingleValue",
			destinations: []ApplicationDestination{{Server: "foo"}},
			want:         []string{"foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := AppProjectSpec{Destinations: tt.destinations}
			if got := d.DestinationClusters(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppProjectSpec.DestinationClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_HasCredentials(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
		want bool
	}{
		{
			name: "TestHasRepo",
			repo: Repository{Repo: "foo"},
			want: false,
		},
		{
			name: "TestHasUsername",
			repo: Repository{Username: "foo"},
			want: true,
		},
		{
			name: "TestHasPassword",
			repo: Repository{Password: "foo"},
			want: true,
		},
		{
			name: "TestHasSSHPrivateKey",
			repo: Repository{SSHPrivateKey: "foo"},
			want: true,
		},
		{
			name: "TestHasTLSClientCertData",
			repo: Repository{TLSClientCertData: "foo"},
			want: true,
		},
		{
			name: "TestHasInsecureHostKey",
			repo: Repository{InsecureIgnoreHostKey: true},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.HasCredentials(); got != tt.want {
				t.Errorf("Repository.HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_IsInsecure(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
		want bool
	}{
		{
			name: "TestHasRepo",
			repo: Repository{Repo: "foo"},
			want: false,
		},
		{
			name: "TestHasUsername",
			repo: Repository{Username: "foo"},
			want: false,
		},
		{
			name: "TestHasInsecure",
			repo: Repository{Insecure: true},
			want: true,
		},
		{
			name: "TestHasInsecureHostKey",
			repo: Repository{InsecureIgnoreHostKey: true},
			want: true,
		},
		{
			name: "TestHasEnableLFS",
			repo: Repository{EnableLFS: true},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.IsInsecure(); got != tt.want {
				t.Errorf("Repository.IsInsecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_IsLFSEnabled(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
		want bool
	}{
		{
			name: "TestHasRepo",
			repo: Repository{Repo: "foo"},
			want: false,
		},
		{
			name: "TestHasUsername",
			repo: Repository{Username: "foo"},
			want: false,
		},
		{
			name: "TestHasInsecure",
			repo: Repository{Insecure: true},
			want: false,
		},
		{
			name: "TestHasInsecureHostKey",
			repo: Repository{InsecureIgnoreHostKey: true},
			want: false,
		},
		{
			name: "TestHasEnableLFS",
			repo: Repository{EnableLFS: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.IsLFSEnabled(); got != tt.want {
				t.Errorf("Repository.IsLFSEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_CopyCredentialsFromRepo(t *testing.T) {
	tests := []struct {
		name   string
		repo   *Repository
		source *Repository
		want   Repository
	}{
		{"Username", &Repository{Username: "foo"}, &Repository{}, Repository{Username: "foo"}},
		{"Password", &Repository{Password: "foo"}, &Repository{}, Repository{Password: "foo"}},
		{"SSHPrivateKey", &Repository{SSHPrivateKey: "foo"}, &Repository{}, Repository{SSHPrivateKey: "foo"}},
		{"InsecureHostKey", &Repository{InsecureIgnoreHostKey: true}, &Repository{}, Repository{InsecureIgnoreHostKey: true}},
		{"Insecure", &Repository{Insecure: true}, &Repository{}, Repository{Insecure: true}},
		{"EnableLFS", &Repository{EnableLFS: true}, &Repository{}, Repository{EnableLFS: true}},
		{"TLSClientCertData", &Repository{TLSClientCertData: "foo"}, &Repository{}, Repository{TLSClientCertData: "foo"}},
		{"TLSClientCertKey", &Repository{TLSClientCertKey: "foo"}, &Repository{}, Repository{TLSClientCertKey: "foo"}},
		{"SourceNil", &Repository{}, nil, Repository{}},

		{"SourceUsername", &Repository{}, &Repository{Username: "foo"}, Repository{Username: "foo"}},
		{"SourcePassword", &Repository{}, &Repository{Password: "foo"}, Repository{Password: "foo"}},
		{"SourceSSHPrivateKey", &Repository{}, &Repository{SSHPrivateKey: "foo"}, Repository{SSHPrivateKey: "foo"}},
		{"SourceInsecureHostKey", &Repository{}, &Repository{InsecureIgnoreHostKey: true}, Repository{InsecureIgnoreHostKey: false}},
		{"SourceInsecure", &Repository{}, &Repository{Insecure: true}, Repository{Insecure: false}},
		{"SourceEnableLFS", &Repository{}, &Repository{EnableLFS: true}, Repository{EnableLFS: false}},
		{"SourceTLSClientCertData", &Repository{}, &Repository{TLSClientCertData: "foo"}, Repository{TLSClientCertData: "foo"}},
		{"SourceTLSClientCertKey", &Repository{}, &Repository{TLSClientCertKey: "foo"}, Repository{TLSClientCertKey: "foo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.repo.DeepCopy()
			r.CopyCredentialsFromRepo(tt.source)
			assert.Equal(t, tt.want, *r)
		})
	}
}

func TestRepository_CopyCredentialsFrom(t *testing.T) {
	tests := []struct {
		name   string
		repo   *Repository
		source *RepoCreds
		want   Repository
	}{
		{"Username", &Repository{Username: "foo"}, &RepoCreds{}, Repository{Username: "foo"}},
		{"Password", &Repository{Password: "foo"}, &RepoCreds{}, Repository{Password: "foo"}},
		{"SSHPrivateKey", &Repository{SSHPrivateKey: "foo"}, &RepoCreds{}, Repository{SSHPrivateKey: "foo"}},
		{"InsecureHostKey", &Repository{InsecureIgnoreHostKey: true}, &RepoCreds{}, Repository{InsecureIgnoreHostKey: true}},
		{"Insecure", &Repository{Insecure: true}, &RepoCreds{}, Repository{Insecure: true}},
		{"EnableLFS", &Repository{EnableLFS: true}, &RepoCreds{}, Repository{EnableLFS: true}},
		{"TLSClientCertData", &Repository{TLSClientCertData: "foo"}, &RepoCreds{}, Repository{TLSClientCertData: "foo"}},
		{"TLSClientCertKey", &Repository{TLSClientCertKey: "foo"}, &RepoCreds{}, Repository{TLSClientCertKey: "foo"}},
		{"SourceNil", &Repository{}, nil, Repository{}},

		{"SourceUsername", &Repository{}, &RepoCreds{Username: "foo"}, Repository{Username: "foo"}},
		{"SourcePassword", &Repository{}, &RepoCreds{Password: "foo"}, Repository{Password: "foo"}},
		{"SourceSSHPrivateKey", &Repository{}, &RepoCreds{SSHPrivateKey: "foo"}, Repository{SSHPrivateKey: "foo"}},
		{"SourceTLSClientCertData", &Repository{}, &RepoCreds{TLSClientCertData: "foo"}, Repository{TLSClientCertData: "foo"}},
		{"SourceTLSClientCertKey", &Repository{}, &RepoCreds{TLSClientCertKey: "foo"}, Repository{TLSClientCertKey: "foo"}},
		{"SourceContainsProxy", &Repository{}, &RepoCreds{Proxy: "http://proxy.argoproj.io:3128", NoProxy: ".example.com"}, Repository{Proxy: "http://proxy.argoproj.io:3128", NoProxy: ".example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.repo.DeepCopy()
			r.CopyCredentialsFrom(tt.source)
			assert.Equal(t, tt.want, *r)
		})
	}
}

func TestRepository_CopySettingsFrom(t *testing.T) {
	tests := []struct {
		name   string
		source *Repository
		want   Repository
	}{
		{"TestNil", nil, Repository{}},
		{"TestHasRepo", &Repository{Repo: "foo"}, Repository{}},
		{"TestHasEnableLFS", &Repository{EnableLFS: true}, Repository{EnableLFS: true}},
		{"TestHasInsecure", &Repository{Insecure: true}, Repository{Insecure: true}},
		{"TestHasInsecureIgnoreHostKey", &Repository{InsecureIgnoreHostKey: true}, Repository{InsecureIgnoreHostKey: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := Repository{}
			repo.CopySettingsFrom(tt.source)
			assert.Equal(t, tt.want, repo)
		})
	}
}

func TestSyncStrategy_Force(t *testing.T) {
	type fields struct {
		Apply *SyncStrategyApply
		Hook  *SyncStrategyHook
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"TestZero", fields{}, false},
		{"TestApply", fields{Apply: &SyncStrategyApply{}}, false},
		{"TestForceApply", fields{Apply: &SyncStrategyApply{Force: true}}, true},
		{"TestHook", fields{Hook: &SyncStrategyHook{}}, false},
		{"TestForceHook", fields{Hook: &SyncStrategyHook{SyncStrategyApply{Force: true}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &SyncStrategy{
				Apply: tt.fields.Apply,
				Hook:  tt.fields.Hook,
			}
			if got := m.Force(); got != tt.want {
				t.Errorf("SyncStrategy.Force() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncOperation_IsApplyStrategy(t *testing.T) {
	type fields struct {
		SyncStrategy *SyncStrategy
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"TestZero", fields{}, false},
		{"TestSyncStrategy", fields{SyncStrategy: &SyncStrategy{}}, false},
		{"TestApplySyncStrategy", fields{SyncStrategy: &SyncStrategy{Apply: &SyncStrategyApply{}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &SyncOperation{
				SyncStrategy: tt.fields.SyncStrategy,
			}
			if got := o.IsApplyStrategy(); got != tt.want {
				t.Errorf("SyncOperation.IsApplyStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceResults_Find(t *testing.T) {
	type args struct {
		group     string
		kind      string
		namespace string
		name      string
		phase     common.SyncPhase
	}
	foo := &ResourceResult{Group: "foo"}
	results := ResourceResults{
		&ResourceResult{Group: "bar"},
		foo,
	}
	tests := []struct {
		name  string
		r     ResourceResults
		args  args
		want  int
		want1 *ResourceResult
	}{
		{"TestNil", nil, args{}, 0, nil},
		{"TestNotFound", results, args{}, 0, nil},
		{"TestFound", results, args{group: "foo"}, 1, foo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.r.Find(tt.args.group, tt.args.kind, tt.args.namespace, tt.args.name, tt.args.phase)
			if got != tt.want {
				t.Errorf("ResourceResults.Find() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ResourceResults.Find() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestResourceResults_PruningRequired(t *testing.T) {
	needsPruning := &ResourceResult{Status: common.ResultCodePruneSkipped}
	tests := []struct {
		name    string
		r       ResourceResults
		wantNum int
	}{
		{"TestNil", ResourceResults{}, 0},
		{"TestOne", ResourceResults{needsPruning}, 1},
		{"TestTwo", ResourceResults{needsPruning, needsPruning}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotNum := tt.r.PruningRequired(); gotNum != tt.wantNum {
				t.Errorf("ResourceResults.PruningRequired() = %v, want %v", gotNum, tt.wantNum)
			}
		})
	}
}

func TestApplicationSource_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSource
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSource{}, true},
		{"RepoURL", &ApplicationSource{RepoURL: "foo"}, false},
		{"Path", &ApplicationSource{Path: "foo"}, false},
		{"TargetRevision", &ApplicationSource{TargetRevision: "foo"}, false},
		{"Helm", &ApplicationSource{Helm: &ApplicationSourceHelm{ReleaseName: "foo"}}, false},
		{"Kustomize", &ApplicationSource{Kustomize: &ApplicationSourceKustomize{Images: KustomizeImages{""}}}, false},
		{"Directory", &ApplicationSource{Directory: &ApplicationSourceDirectory{Recurse: true}}, false},
		{"Plugin", &ApplicationSource{Plugin: &ApplicationSourcePlugin{Name: "foo"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestApplicationSourceHelm_AddParameter(t *testing.T) {
	src := ApplicationSourceHelm{}
	t.Run("Add", func(t *testing.T) {
		src.AddParameter(HelmParameter{Value: "bar"})
		assert.ElementsMatch(t, []HelmParameter{{Value: "bar"}}, src.Parameters)
	})
	t.Run("Replace", func(t *testing.T) {
		src.AddParameter(HelmParameter{Value: "baz"})
		assert.ElementsMatch(t, []HelmParameter{{Value: "baz"}}, src.Parameters)
	})
}

func TestApplicationSourceHelm_AddFileParameter(t *testing.T) {
	src := ApplicationSourceHelm{}
	t.Run("Add", func(t *testing.T) {
		src.AddFileParameter(HelmFileParameter{Name: "foo", Path: "bar"})
		assert.ElementsMatch(t, []HelmFileParameter{{Name: "foo", Path: "bar"}}, src.FileParameters)
	})
	t.Run("Replace", func(t *testing.T) {
		src.AddFileParameter(HelmFileParameter{Name: "foo", Path: "baz"})
		assert.ElementsMatch(t, []HelmFileParameter{{Name: "foo", Path: "baz"}}, src.FileParameters)
	})
}

func TestNewHelmParameter(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := NewHelmParameter("garbage", false)
		require.EqualError(t, err, "Expected helm parameter of the form: param=value. Received: garbage")
	})
	t.Run("NonString", func(t *testing.T) {
		p, err := NewHelmParameter("foo=bar", false)
		require.NoError(t, err)
		assert.Equal(t, &HelmParameter{Name: "foo", Value: "bar"}, p)
	})
	t.Run("String", func(t *testing.T) {
		p, err := NewHelmParameter("foo=bar", true)
		require.NoError(t, err)
		assert.Equal(t, &HelmParameter{Name: "foo", Value: "bar", ForceString: true}, p)
	})
}

func TestNewKustomizeReplica(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		r, err := NewKustomizeReplica("my-deployment=2")
		require.NoError(t, err)
		assert.Equal(t, &KustomizeReplica{Name: "my-deployment", Count: intstr.Parse("2")}, r)
	})
	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := NewKustomizeReplica("garbage")
		require.EqualError(t, err, "expected parameter of the form: name=count. Received: garbage")
	})
	t.Run("InvalidCount", func(t *testing.T) {
		_, err := NewKustomizeReplica("my-deployment=garbage")
		require.EqualError(t, err, "expected integer value for count. Received: garbage")
	})
}

func TestKustomizeReplica_GetIntCount(t *testing.T) {
	t.Run("String which can be converted to integer", func(t *testing.T) {
		kr := KustomizeReplica{
			Name:  "test",
			Count: intstr.FromString("2"),
		}
		count, err := kr.GetIntCount()
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
	t.Run("String which cannot be converted to integer", func(t *testing.T) {
		kr := KustomizeReplica{
			Name:  "test",
			Count: intstr.FromString("garbage"),
		}
		count, err := kr.GetIntCount()
		require.EqualError(t, err, "expected integer value for count. Received: garbage")
		assert.Equal(t, 0, count)
	})
	t.Run("Integer", func(t *testing.T) {
		kr := KustomizeReplica{
			Name:  "test",
			Count: intstr.FromInt(2),
		}
		count, err := kr.GetIntCount()
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestApplicationSourceKustomize_MergeReplica(t *testing.T) {
	r1 := KustomizeReplica{
		Name:  "my-deployment",
		Count: intstr.FromInt(2),
	}
	r2 := KustomizeReplica{
		Name:  "my-deployment",
		Count: intstr.FromInt(4),
	}
	t.Run("Add", func(t *testing.T) {
		k := ApplicationSourceKustomize{Replicas: KustomizeReplicas{}}
		k.MergeReplica(r1)
		assert.Equal(t, KustomizeReplicas{r1}, k.Replicas)
	})
	t.Run("Replace", func(t *testing.T) {
		k := ApplicationSourceKustomize{Replicas: KustomizeReplicas{r1}}
		k.MergeReplica(r2)
		assert.Len(t, k.Replicas, 1)
		assert.Equal(t, k.Replicas[0].Name, r2.Name)
		assert.Equal(t, k.Replicas[0].Count, r2.Count)
	})
}

func TestApplicationSourceKustomize_FindByName(t *testing.T) {
	r1 := KustomizeReplica{
		Name:  "my-deployment",
		Count: intstr.FromInt(2),
	}
	r2 := KustomizeReplica{
		Name:  "my-statefulset",
		Count: intstr.FromInt(4),
	}
	Replicas := KustomizeReplicas{r1, r2}
	t.Run("Found", func(t *testing.T) {
		i1 := Replicas.FindByName("my-deployment")
		i2 := Replicas.FindByName("my-statefulset")
		assert.Equal(t, 0, i1)
		assert.Equal(t, 1, i2)
	})
	t.Run("Not Found", func(t *testing.T) {
		i := Replicas.FindByName("not-found")
		assert.Equal(t, -1, i)
	})
}

func TestApplicationSourceHelm_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSourceHelm
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSourceHelm{}, true},
		{"ValueFiles", &ApplicationSourceHelm{ValueFiles: []string{""}}, false},
		{"Parameters", &ApplicationSourceHelm{Parameters: []HelmParameter{{}}}, false},
		{"ReleaseName", &ApplicationSourceHelm{ReleaseName: "foa"}, false},
		{"FileParameters", &ApplicationSourceHelm{FileParameters: []HelmFileParameter{{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestApplicationSourceKustomize_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSourceKustomize
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSourceKustomize{}, true},
		{"NamePrefix", &ApplicationSourceKustomize{NamePrefix: "foo"}, false},
		{"NameSuffix", &ApplicationSourceKustomize{NameSuffix: "foo"}, false},
		{"Images", &ApplicationSourceKustomize{Images: []KustomizeImage{""}}, false},
		{"Replicas", &ApplicationSourceKustomize{Replicas: []KustomizeReplica{{Name: "", Count: intstr.FromInt(0)}}}, false},
		{"CommonLabels", &ApplicationSourceKustomize{CommonLabels: map[string]string{"": ""}}, false},
		{"CommonAnnotations", &ApplicationSourceKustomize{CommonAnnotations: map[string]string{"": ""}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestApplicationSourceJsonnet_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSourceJsonnet
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSourceJsonnet{}, true},
		{"ExtVars", &ApplicationSourceJsonnet{ExtVars: []JsonnetVar{{}}}, false},
		{"TLAs", &ApplicationSourceJsonnet{TLAs: []JsonnetVar{{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestApplicationSourceDirectory_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSourceDirectory
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSourceDirectory{}, true},
		{"Recurse", &ApplicationSourceDirectory{Recurse: true}, false},
		{"Jsonnet", &ApplicationSourceDirectory{Jsonnet: ApplicationSourceJsonnet{ExtVars: []JsonnetVar{{}}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestApplicationSourcePlugin_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		source *ApplicationSourcePlugin
		want   bool
	}{
		{"Nil", nil, true},
		{"Empty", &ApplicationSourcePlugin{}, true},
		{"Name", &ApplicationSourcePlugin{Name: "foo"}, false},
		{"Env", &ApplicationSourcePlugin{Env: Env{{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.source.IsZero())
		})
	}
}

func TestEnvEntry_IsZero(t *testing.T) {
	tests := []struct {
		name string
		env  *EnvEntry
		want bool
	}{
		{"Nil", nil, true},
		{"Empty", &EnvEntry{}, true},
		{"Name", &EnvEntry{Name: "FOO"}, false},
		{"Value", &EnvEntry{Value: "foo"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.env.IsZero())
		})
	}
}

func TestEnv_IsZero(t *testing.T) {
	tests := []struct {
		name string
		e    Env
		want bool
	}{
		{"Empty", Env{}, true},
		{"One", Env{{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.e.IsZero())
		})
	}
}

func TestEnv_Envsubst(t *testing.T) {
	env := Env{&EnvEntry{"FOO", "bar"}}
	assert.Equal(t, "", env.Envsubst(""))
	assert.Equal(t, "bar", env.Envsubst("$FOO"))
	assert.Equal(t, "bar", env.Envsubst("${FOO}"))
	assert.Equal(t, "FOO", env.Envsubst("${FOO"))
	assert.Equal(t, "", env.Envsubst("$BAR"))
	assert.Equal(t, "", env.Envsubst("${BAR}"))
	assert.Equal(t,
		"echo bar; echo ; echo bar; echo ; echo FOO",
		env.Envsubst("echo $FOO; echo $BAR; echo ${FOO}; echo ${BAR}; echo ${FOO"),
	)
}

func TestEnv_Envsubst_Overlap(t *testing.T) {
	env := Env{&EnvEntry{"ARGOCD_APP_NAMESPACE", "default"}, &EnvEntry{"ARGOCD_APP_NAME", "guestbook"}}

	assert.Equal(t,
		"namespace: default; name: guestbook",
		env.Envsubst("namespace: $ARGOCD_APP_NAMESPACE; name: $ARGOCD_APP_NAME"),
	)
}

func TestEnv_Environ(t *testing.T) {
	tests := []struct {
		name string
		e    Env
		want []string
	}{
		{"Nil", nil, nil},
		{"Env", Env{{}}, nil},
		{"One", Env{{"FOO", "bar"}}, []string{"FOO=bar"}},
		{"Two", Env{{"FOO", "bar"}, {"FOO", "bar"}}, []string{"FOO=bar", "FOO=bar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.e.Environ())
		})
	}
}

func TestKustomizeImage_Match(t *testing.T) {
	// no prefix
	assert.False(t, KustomizeImage("foo=1").Match("bar=1"))
	// mismatched delimiter
	assert.False(t, KustomizeImage("foo=1").Match("bar:1"))
	assert.False(t, KustomizeImage("foo:1").Match("bar=1"))
	// matches
	assert.True(t, KustomizeImage("foo=1").Match("foo=2"))
	assert.True(t, KustomizeImage("foo:1").Match("foo:2"))
	assert.True(t, KustomizeImage("foo@1").Match("foo@2"))
}

func TestApplicationSourceKustomize_MergeImage(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		k := ApplicationSourceKustomize{Images: KustomizeImages{}}
		k.MergeImage("foo=1")
		assert.Equal(t, KustomizeImages{"foo=1"}, k.Images)
	})
	t.Run("Replace", func(t *testing.T) {
		k := ApplicationSourceKustomize{Images: KustomizeImages{"foo=1"}}
		k.MergeImage("foo=2")
		assert.Equal(t, KustomizeImages{"foo=2"}, k.Images)
	})
}

func TestSyncWindows_HasWindows(t *testing.T) {
	t.Run("True", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		assert.True(t, proj.Spec.SyncWindows.HasWindows())
	})
	t.Run("False", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		err := proj.Spec.DeleteWindow(0)
		require.NoError(t, err)
		assert.False(t, proj.Spec.SyncWindows.HasWindows())
	})
}

func TestSyncWindows_Active(t *testing.T) {
	t.Run("WithTestProject", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		assert.Len(t, *proj.Spec.SyncWindows.Active(), 1)
	})

	syncWindow := func(kind string, schedule string, duration string, timeZone string) *SyncWindow {
		return &SyncWindow{
			Kind:         kind,
			Schedule:     schedule,
			Duration:     duration,
			Applications: []string{},
			Namespaces:   []string{},
			TimeZone:     timeZone,
		}
	}

	timeWithHour := func(hour int, location *time.Location) time.Time {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, location)
	}

	utcM4Zone := time.FixedZone("UTC-4", -4*60*60)

	tests := []struct {
		name           string
		syncWindow     SyncWindows
		currentTime    time.Time
		matchingIndex  int
		expectedLength int
	}{
		{
			name: "MatchFirst",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(11, time.UTC),
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(15, time.UTC),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchBoth",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "3h", ""),
				syncWindow("allow", "* 11 * * *", "3h", ""),
			},
			currentTime:    timeWithHour(12, time.UTC),
			expectedLength: 2,
		},
		{
			name: "MatchNone",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(17, time.UTC),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(11-4, utcM4Zone), // 11AM UTC is 7AM EDT
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(15-4, utcM4Zone),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchNone-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(17-4, utcM4Zone),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 14 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(16, time.UTC),
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 14 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(20, time.UTC),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchNone-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 14 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(22, time.UTC),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-PositiveTimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 8 * * *", "2h", "Asia/Dhaka"),
				syncWindow("allow", "* 12 * * *", "2h", "Asia/Dhaka"),
			},
			currentTime:    timeWithHour(3, time.UTC),
			matchingIndex:  0,
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.syncWindow.active(tt.currentTime)
			if result == nil {
				result = &SyncWindows{}
			}
			assert.Len(t, *result, tt.expectedLength)

			if len(*result) == 1 {
				assert.Equal(t, tt.syncWindow[tt.matchingIndex], (*result)[0])
			}
		})
	}
}

func TestSyncWindows_InactiveAllows(t *testing.T) {
	t.Run("WithTestProject", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		proj.Spec.SyncWindows[0].Schedule = "0 0 1 1 1"
		assert.Len(t, *proj.Spec.SyncWindows.InactiveAllows(), 1)
	})

	syncWindow := func(kind string, schedule string, duration string, timeZone string) *SyncWindow {
		return &SyncWindow{
			Kind:         kind,
			Schedule:     schedule,
			Duration:     duration,
			Applications: []string{},
			Namespaces:   []string{},
			TimeZone:     timeZone,
		}
	}

	timeWithHour := func(hour int, location *time.Location) time.Time {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, location)
	}

	utcM4Zone := time.FixedZone("UTC-4", -4*60*60)

	tests := []struct {
		name           string
		syncWindow     SyncWindows
		currentTime    time.Time
		matchingIndex  int
		expectedLength int
	}{
		{
			name: "MatchFirst",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 5 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(6, time.UTC),
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(11, time.UTC),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchBoth",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(17, time.UTC),
			expectedLength: 2,
		},
		{
			name: "MatchNone",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "4h", ""),
				syncWindow("allow", "* 11 * * *", "4h", ""),
			},
			currentTime:    timeWithHour(12, time.UTC),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 5 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(6-4, utcM4Zone), // 6AM UTC is 2AM EDT
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(11-4, utcM4Zone),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchBoth-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", ""),
				syncWindow("allow", "* 14 * * *", "2h", ""),
			},
			currentTime:    timeWithHour(17-4, utcM4Zone),
			expectedLength: 2,
		},
		{
			name: "MatchNone-NonUTC",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "4h", ""),
				syncWindow("allow", "* 11 * * *", "4h", ""),
			},
			currentTime:    timeWithHour(12-4, utcM4Zone),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 5 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(11, time.UTC), // 6AM UTC is 2AM EDT
			matchingIndex:  0,
			expectedLength: 1,
		},
		{
			name: "MatchSecond-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 14 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(16, time.UTC),
			matchingIndex:  1,
			expectedLength: 1,
		},
		{
			name: "MatchBoth-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "2h", "America/New_York"),
				syncWindow("allow", "* 14 * * *", "2h", "America/New_York"),
			},
			currentTime:    timeWithHour(6, time.UTC),
			expectedLength: 2,
		},
		{
			name: "MatchNone-TimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 10 * * *", "4h", ""),
				syncWindow("allow", "* 11 * * *", "4h", ""),
			},
			currentTime:    timeWithHour(12, time.UTC),
			expectedLength: 0,
		},
		{
			name: "MatchFirst-PositiveTimeZoneSpecified",
			syncWindow: SyncWindows{
				syncWindow("allow", "* 8 * * *", "2h", "Asia/Dhaka"),
				syncWindow("allow", "* 12 * * *", "2h", "Asia/Dhaka"),
			},
			currentTime:    timeWithHour(7, time.UTC),
			matchingIndex:  0,
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.syncWindow.inactiveAllows(tt.currentTime)
			if result == nil {
				result = &SyncWindows{}
			}
			assert.Len(t, *result, tt.expectedLength)

			if len(*result) == 1 {
				assert.Equal(t, tt.syncWindow[tt.matchingIndex], (*result)[0])
			}
		})
	}
}

func TestAppProjectSpec_AddWindow(t *testing.T) {
	proj := newTestProjectWithSyncWindows()
	tests := []struct {
		name string
		p    *AppProject
		k    string
		s    string
		d    string
		a    []string
		n    []string
		c    []string
		m    bool
		t    string
		want string
	}{
		{"MissingKind", proj, "", "* * * * *", "11", []string{"app1"}, []string{}, []string{}, false, "error", ""},
		{"MissingSchedule", proj, "allow", "", "", []string{"app1"}, []string{}, []string{}, false, "error", ""},
		{"MissingDuration", proj, "allow", "* * * * *", "", []string{"app1"}, []string{}, []string{}, false, "error", ""},
		{"BadSchedule", proj, "allow", "* * *", "1h", []string{"app1"}, []string{}, []string{}, false, "error", ""},
		{"BadDuration", proj, "deny", "* * * * *", "33mm", []string{"app1"}, []string{}, []string{}, false, "error", ""},
		{"WorkingApplication", proj, "allow", "1 * * * *", "1h", []string{"app1"}, []string{}, []string{}, false, "noError", ""},
		{"WorkingNamespace", proj, "deny", "3 * * * *", "1h", []string{}, []string{}, []string{"cluster"}, false, "noError", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.want {
			case "error":
				require.Error(t, tt.p.Spec.AddWindow(tt.k, tt.s, tt.d, tt.a, tt.n, tt.c, tt.m, tt.t))
			case "noError":
				require.NoError(t, tt.p.Spec.AddWindow(tt.k, tt.s, tt.d, tt.a, tt.n, tt.c, tt.m, tt.t))
				require.NoError(t, tt.p.Spec.DeleteWindow(0))
			}
		})
	}
}

func TestAppProjectSpec_DeleteWindow(t *testing.T) {
	proj := newTestProjectWithSyncWindows()
	window2 := &SyncWindow{Schedule: "1 * * * *", Duration: "2h"}
	proj.Spec.SyncWindows = append(proj.Spec.SyncWindows, window2)
	t.Run("CannotFind", func(t *testing.T) {
		err := proj.Spec.DeleteWindow(3)
		require.Error(t, err)
		assert.Len(t, proj.Spec.SyncWindows, 2)
	})
	t.Run("Delete", func(t *testing.T) {
		err := proj.Spec.DeleteWindow(0)
		require.NoError(t, err)
		assert.Len(t, proj.Spec.SyncWindows, 1)
	})
}

func TestSyncWindows_Matches(t *testing.T) {
	proj := newTestProjectWithSyncWindows()
	app := newTestApp()
	t.Run("MatchNamespace", func(t *testing.T) {
		proj.Spec.SyncWindows[0].Namespaces = []string{"default"}
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Len(t, *windows, 1)
		proj.Spec.SyncWindows[0].Namespaces = nil
	})
	t.Run("MatchCluster", func(t *testing.T) {
		proj.Spec.SyncWindows[0].Clusters = []string{"cluster1"}
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Len(t, *windows, 1)
		proj.Spec.SyncWindows[0].Clusters = nil
	})
	t.Run("MatchClusterName", func(t *testing.T) {
		proj.Spec.SyncWindows[0].Clusters = []string{"clusterName"}
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Len(t, *windows, 1)
		proj.Spec.SyncWindows[0].Clusters = nil
	})
	t.Run("MatchAppName", func(t *testing.T) {
		proj.Spec.SyncWindows[0].Applications = []string{"test-app"}
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Len(t, *windows, 1)
		proj.Spec.SyncWindows[0].Applications = nil
	})
	t.Run("MatchWildcardAppName", func(t *testing.T) {
		proj.Spec.SyncWindows[0].Applications = []string{"test-*"}
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Len(t, *windows, 1)
		proj.Spec.SyncWindows[0].Applications = nil
	})
	t.Run("NoMatch", func(t *testing.T) {
		windows := proj.Spec.SyncWindows.Matches(app)
		assert.Nil(t, windows)
	})
}

func TestSyncWindows_CanSync(t *testing.T) {
	t.Run("will allow manual sync if inactive-deny-window set with manual true", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().withInactiveDenyWindow(true).build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will allow manual sync if inactive-deny-window set with manual false", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().withInactiveDenyWindow(false).build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny manual sync if one inactive-allow-windows set with manual false", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(true).
			withInactiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will allow manual sync if on active-allow-window set with manual true", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will allow manual sync if on active-allow-window set with manual false", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will allow auto sync if on active-allow-window", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.True(t, canSync)
	})
	t.Run("will allow manual sync active-allow and inactive-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withInactiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will allow auto sync active-allow and inactive-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withInactiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny manual sync inactive-allow", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny auto sync inactive-allow", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will allow manual sync inactive-allow with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny auto sync inactive-allow with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny manual sync with inactive-allow and inactive-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(false).
			withInactiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny auto sync with inactive-allow and inactive-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withInactiveAllowWindow(false).
			withInactiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will allow auto sync with active-allow and inactive-allow", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withInactiveAllowWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny manual sync with active-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny auto sync with active-deny", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will allow manual sync with active-deny with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny auto sync with active-deny with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny manual sync with many active-deny having one with ManualSync disabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(true).
			withActiveDenyWindow(true).
			withActiveDenyWindow(true).
			withActiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny auto sync with many active-deny having one with ManualSync disabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveDenyWindow(true).
			withActiveDenyWindow(true).
			withActiveDenyWindow(true).
			withActiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
	t.Run("will deny manual sync with active-deny and active-allow windows with ManualSync disabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withActiveDenyWindow(false).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.False(t, canSync)
	})
	t.Run("will allow manual sync with active-deny and active-allow windows with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withActiveDenyWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(true)

		// then
		assert.True(t, canSync)
	})
	t.Run("will deny auto sync with active-deny and active-allow windows with ManualSync enabled", func(t *testing.T) {
		// given
		t.Parallel()
		proj := newProjectBuilder().
			withActiveAllowWindow(false).
			withActiveDenyWindow(true).
			build()

		// when
		canSync := proj.Spec.SyncWindows.CanSync(false)

		// then
		assert.False(t, canSync)
	})
}

func TestSyncWindows_hasDeny(t *testing.T) {
	t.Run("True", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		deny := &SyncWindow{Kind: "deny"}
		proj.Spec.SyncWindows = append(proj.Spec.SyncWindows, deny)
		hasDeny, manualEnabled := proj.Spec.SyncWindows.hasDeny()
		assert.True(t, hasDeny)
		assert.False(t, manualEnabled)
	})
	t.Run("TrueManualEnabled", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		deny := &SyncWindow{Kind: "deny", ManualSync: true}
		proj.Spec.SyncWindows = append(proj.Spec.SyncWindows, deny)
		hasDeny, manualEnabled := proj.Spec.SyncWindows.hasDeny()
		assert.True(t, hasDeny)
		assert.True(t, manualEnabled)
	})
	t.Run("False", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		hasDeny, manualEnabled := proj.Spec.SyncWindows.hasDeny()
		assert.False(t, hasDeny)
		assert.False(t, manualEnabled)
	})
}

func TestSyncWindows_hasAllow(t *testing.T) {
	t.Run("NoWindows", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		_ = proj.Spec.DeleteWindow(0)
		assert.False(t, proj.Spec.SyncWindows.hasAllow())
	})
	t.Run("True", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		assert.True(t, proj.Spec.SyncWindows.hasAllow())
	})
	t.Run("NoWindows", func(t *testing.T) {
		proj := newTestProjectWithSyncWindows()
		proj.Spec.SyncWindows[0].Kind = "deny"
		assert.False(t, proj.Spec.SyncWindows.hasAllow())
	})
}

func TestSyncWindow_Active(t *testing.T) {
	window := &SyncWindow{Schedule: "* * * * *", Duration: "1h"}
	t.Run("ActiveWindow", func(t *testing.T) {
		window.Active()
		assert.True(t, window.Active())
	})

	syncWindow := func(kind string, schedule string, duration string) SyncWindow {
		return SyncWindow{
			Kind:         kind,
			Schedule:     schedule,
			Duration:     duration,
			Applications: []string{},
			Namespaces:   []string{},
		}
	}

	timeWithHour := func(hour int, location *time.Location) time.Time {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, location)
	}

	utcM4Zone := time.FixedZone("UTC-4", -4*60*60) // Eastern Daylight Saving Time (EDT)

	tests := []struct {
		name           string
		syncWindow     SyncWindow
		currentTime    time.Time
		expectedResult bool
	}{
		{
			name:           "Allow-active",
			syncWindow:     syncWindow("allow", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(11, time.UTC),
			expectedResult: true,
		},
		{
			name:           "Allow-inactive",
			syncWindow:     syncWindow("allow", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(13, time.UTC),
			expectedResult: false,
		},
		{
			name:           "Deny-active",
			syncWindow:     syncWindow("deny", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(11, time.UTC),
			expectedResult: true,
		},
		{
			name:           "Deny-inactive",
			syncWindow:     syncWindow("deny", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(13, time.UTC),
			expectedResult: false,
		},
		{
			name:           "Allow-active-NonUTC",
			syncWindow:     syncWindow("allow", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(11-4, utcM4Zone), // 11AM UTC is 7AM EDT
			expectedResult: true,
		},
		{
			name:           "Allow-inactive-NonUTC",
			syncWindow:     syncWindow("allow", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(13-4, utcM4Zone),
			expectedResult: false,
		},
		{
			name:           "Deny-active-NonUTC",
			syncWindow:     syncWindow("deny", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(11-4, utcM4Zone),
			expectedResult: true,
		},
		{
			name:           "Deny-inactive-NonUTC",
			syncWindow:     syncWindow("deny", "* 10 * * *", "2h"),
			currentTime:    timeWithHour(13-4, utcM4Zone),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.syncWindow.active(tt.currentTime)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSyncWindow_Update(t *testing.T) {
	e := SyncWindow{Kind: "allow", Schedule: "* * * * *", Duration: "1h", Applications: []string{"app1"}}
	t.Run("AddApplication", func(t *testing.T) {
		err := e.Update("", "", []string{"app1", "app2"}, []string{}, []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"app1", "app2"}, e.Applications)
	})
	t.Run("AddNamespace", func(t *testing.T) {
		err := e.Update("", "", []string{}, []string{"namespace1"}, []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"namespace1"}, e.Namespaces)
	})
	t.Run("AddCluster", func(t *testing.T) {
		err := e.Update("", "", []string{}, []string{}, []string{"cluster1"}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"cluster1"}, e.Clusters)
	})
	t.Run("MissingConfig", func(t *testing.T) {
		err := e.Update("", "", []string{}, []string{}, []string{}, "")
		require.EqualError(t, err, "cannot update: require one or more of schedule, duration, application, namespace, or cluster")
	})
	t.Run("ChangeDuration", func(t *testing.T) {
		err := e.Update("", "10h", []string{}, []string{}, []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, "10h", e.Duration)
	})
	t.Run("ChangeSchedule", func(t *testing.T) {
		err := e.Update("* 1 0 0 *", "", []string{}, []string{}, []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, "* 1 0 0 *", e.Schedule)
	})
}

func TestSyncWindow_Validate(t *testing.T) {
	window := &SyncWindow{Kind: "allow", Schedule: "* * * * *", Duration: "1h"}
	t.Run("Validates", func(t *testing.T) {
		require.NoError(t, window.Validate())
	})
	t.Run("IncorrectKind", func(t *testing.T) {
		window.Kind = "wrong"
		require.Error(t, window.Validate())
	})
	t.Run("IncorrectSchedule", func(t *testing.T) {
		window.Kind = "allow"
		window.Schedule = "* * *"
		require.Error(t, window.Validate())
	})
	t.Run("IncorrectDuration", func(t *testing.T) {
		window.Kind = "allow"
		window.Schedule = "* * * * *"
		window.Duration = "1000days"
		require.Error(t, window.Validate())
	})
}

func TestApplicationStatus_GetConditions(t *testing.T) {
	status := ApplicationStatus{
		Conditions: []ApplicationCondition{
			{Type: ApplicationConditionInvalidSpecError},
			{Type: ApplicationConditionRepeatedResourceWarning},
		},
	}
	conditions := status.GetConditions(map[ApplicationConditionType]bool{
		ApplicationConditionInvalidSpecError: true,
	})
	assert.EqualValues(t, []ApplicationCondition{{Type: ApplicationConditionInvalidSpecError}}, conditions)
}

type projectBuilder struct {
	proj *AppProject
}

func newProjectBuilder() *projectBuilder {
	return &projectBuilder{
		proj: newTestProject(),
	}
}

func (b *projectBuilder) build() *AppProject {
	return b.proj
}

func (b *projectBuilder) withActiveAllowWindow(allowManual bool) *projectBuilder {
	window := newSyncWindow("allow", "* * * * *", allowManual)
	b.proj.Spec.SyncWindows = append(b.proj.Spec.SyncWindows, window)
	return b
}

func (b *projectBuilder) withInactiveAllowWindow(allowManual bool) *projectBuilder {
	window := newSyncWindow("allow", inactiveCronSchedule(), allowManual)
	b.proj.Spec.SyncWindows = append(b.proj.Spec.SyncWindows, window)
	return b
}

func (b *projectBuilder) withActiveDenyWindow(allowManual bool) *projectBuilder {
	window := newSyncWindow("deny", "* * * * *", allowManual)
	b.proj.Spec.SyncWindows = append(b.proj.Spec.SyncWindows, window)
	return b
}

func (b *projectBuilder) withInactiveDenyWindow(allowManual bool) *projectBuilder {
	window := newSyncWindow("deny", inactiveCronSchedule(), allowManual)
	b.proj.Spec.SyncWindows = append(b.proj.Spec.SyncWindows, window)
	return b
}

func inactiveCronSchedule() string {
	hourPlus10, _, _ := time.Now().Add(10 * time.Hour).Clock()
	return fmt.Sprintf("0 %d * * *", hourPlus10)
}

func newSyncWindow(kind, schedule string, allowManual bool) *SyncWindow {
	return &SyncWindow{
		Kind:         kind,
		Schedule:     schedule,
		Duration:     "1h",
		Applications: []string{"app1"},
		Namespaces:   []string{"public"},
		ManualSync:   allowManual,
	}
}

func newTestProjectWithSyncWindows() *AppProject {
	return newProjectBuilder().withActiveAllowWindow(false).build()
}

func newTestApp() *Application {
	a := &Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app"},
		Spec: ApplicationSpec{
			Destination: ApplicationDestination{
				Namespace: "default",
				Server:    "cluster1",
				Name:      "clusterName",
			},
		},
	}
	return a
}

func TestNewJsonnetVar(t *testing.T) {
	assert.Equal(t, JsonnetVar{}, NewJsonnetVar("", false))
	assert.Equal(t, JsonnetVar{Name: "a"}, NewJsonnetVar("a=", false))
	assert.Equal(t, JsonnetVar{Name: "a", Code: true}, NewJsonnetVar("a=", true))
	assert.Equal(t, JsonnetVar{Name: "a", Value: "b", Code: true}, NewJsonnetVar("a=b", true))
}

func testCond(t ApplicationConditionType, msg string, lastTransitionTime *metav1.Time) ApplicationCondition {
	return ApplicationCondition{
		Type:               t,
		Message:            msg,
		LastTransitionTime: lastTransitionTime,
	}
}

func TestSetConditions(t *testing.T) {
	fiveMinsAgo := &metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	tenMinsAgo := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	tests := []struct {
		name           string
		existing       []ApplicationCondition
		incoming       []ApplicationCondition
		evaluatedTypes map[ApplicationConditionType]bool
		expected       []ApplicationCondition
		validate       func(*testing.T, *Application)
	}{
		{
			name:     "new conditions with lastTransitionTime",
			existing: []ApplicationCondition{},
			incoming: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			validate: func(t *testing.T, a *Application) {
				assert.Equal(t, fiveMinsAgo, a.Status.Conditions[0].LastTransitionTime)
				assert.Equal(t, tenMinsAgo, a.Status.Conditions[1].LastTransitionTime)
			},
		},
		{
			name:     "new conditions without lastTransitionTime",
			existing: []ApplicationCondition{},
			incoming: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", nil),
				testCond(ApplicationConditionSharedResourceWarning, "bar", nil),
			},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", nil),
				testCond(ApplicationConditionSharedResourceWarning, "bar", nil),
			},
			validate: func(t *testing.T, a *Application) {
				// SetConditions should add timestamps for new conditions.
				assert.True(t, a.Status.Conditions[0].LastTransitionTime.Time.After(fiveMinsAgo.Time))
				assert.True(t, a.Status.Conditions[1].LastTransitionTime.Time.After(fiveMinsAgo.Time))
			},
		},
		{
			name: "condition cleared",
			existing: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			incoming: []ApplicationCondition{
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			validate: func(t *testing.T, a *Application) {
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[0].LastTransitionTime.Time)
			},
		},
		{
			name: "all conditions cleared",
			existing: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			incoming: []ApplicationCondition{},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{},
		},
		{
			name: "existing condition lastTransitionTime preserved",
			existing: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", tenMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			incoming: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", fiveMinsAgo),
			},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", tenMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			validate: func(t *testing.T, a *Application) {
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[0].LastTransitionTime.Time)
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[1].LastTransitionTime.Time)
			},
		},
		{
			name: "existing condition lastTransitionTime updated if message changed",
			existing: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", tenMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			incoming: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar changed message", fiveMinsAgo),
			},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError:      true,
				ApplicationConditionSharedResourceWarning: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", tenMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar changed message", fiveMinsAgo),
			},
			validate: func(t *testing.T, a *Application) {
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[0].LastTransitionTime.Time)
				assert.Equal(t, fiveMinsAgo.Time, a.Status.Conditions[1].LastTransitionTime.Time)
			},
		},
		{
			name: "unevaluated condition types preserved",
			existing: []ApplicationCondition{
				testCond(ApplicationConditionInvalidSpecError, "foo", fiveMinsAgo),
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			incoming: []ApplicationCondition{},
			evaluatedTypes: map[ApplicationConditionType]bool{
				ApplicationConditionInvalidSpecError: true,
			},
			expected: []ApplicationCondition{
				testCond(ApplicationConditionSharedResourceWarning, "bar", tenMinsAgo),
			},
			validate: func(t *testing.T, a *Application) {
				assert.Equal(t, tenMinsAgo.Time, a.Status.Conditions[0].LastTransitionTime.Time)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestApp()
			a.Status.Conditions = tt.existing
			a.Status.SetConditions(tt.incoming, tt.evaluatedTypes)
			assertConditions(t, tt.expected, a.Status.Conditions)
			if tt.validate != nil {
				tt.validate(t, a)
			}
		})
	}
}

// assertConditions compares two arrays of conditions without their timestamps, which may be
// difficult to strictly assert on as they can use time.Now(). Elements in each array are assumed
// to match positions.
func assertConditions(t *testing.T, expected []ApplicationCondition, actual []ApplicationCondition) {
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		assert.Equal(t, expected[i].Type, actual[i].Type)
		assert.Equal(t, expected[i].Message, actual[i].Message)
	}
}

func TestSyncPolicy_IsZero(t *testing.T) {
	var nilPolicy *SyncPolicy
	assert.True(t, nilPolicy.IsZero())
	assert.True(t, (&SyncPolicy{}).IsZero())
	assert.False(t, (&SyncPolicy{Automated: &SyncPolicyAutomated{}}).IsZero())
	assert.False(t, (&SyncPolicy{SyncOptions: SyncOptions{""}}).IsZero())
	assert.False(t, (&SyncPolicy{Retry: &RetryStrategy{}}).IsZero())
}

func TestSyncOptions_HasOption(t *testing.T) {
	var nilOptions SyncOptions
	assert.False(t, nilOptions.HasOption("a=1"))
	assert.False(t, (&SyncOptions{}).HasOption("a=1"))
	assert.True(t, (&SyncOptions{"a=1"}).HasOption("a=1"))
}

func TestSyncOptions_AddOption(t *testing.T) {
	options := SyncOptions{}
	assert.Len(t, options.AddOption("a=1"), 1)
	assert.Len(t, options.AddOption("a=1").AddOption("a=1"), 1)
}

func TestSyncOptions_RemoveOption(t *testing.T) {
	options := SyncOptions{"a=1"}
	assert.Empty(t, options.RemoveOption("a=1"))
	assert.Empty(t, options.RemoveOption("a=1").RemoveOption("a=1"))
}

func TestRevisionHistories_Trunc(t *testing.T) {
	assert.Empty(t, RevisionHistories{}.Trunc(1))
	assert.Len(t, RevisionHistories{{}}.Trunc(1), 1)
	assert.Len(t, RevisionHistories{{}, {}}.Trunc(1), 1)
	// keep the last element, even with longer list
	assert.Equal(t, RevisionHistories{{Revision: "my-revision"}}, RevisionHistories{{}, {}, {Revision: "my-revision"}}.Trunc(1))
}

func TestApplicationSpec_GetRevisionHistoryLimit(t *testing.T) {
	// default
	assert.Equal(t, 10, ApplicationSpec{}.GetRevisionHistoryLimit())
	// configured
	n := int64(11)
	assert.Equal(t, 11, ApplicationSpec{RevisionHistoryLimit: &n}.GetRevisionHistoryLimit())
}

func TestProjectNormalize(t *testing.T) {
	issuedAt := int64(1)
	secondIssuedAt := issuedAt + 1
	thirdIssuedAt := secondIssuedAt + 1
	fourthIssuedAt := thirdIssuedAt + 1

	testTokens := []JWTToken{{ID: "1", IssuedAt: issuedAt}, {ID: "2", IssuedAt: secondIssuedAt}}
	tokensByRole := make(map[string]JWTTokens)
	tokensByRole["test-role"] = JWTTokens{Items: testTokens}

	testTokens2 := []JWTToken{{IssuedAt: issuedAt}, {IssuedAt: thirdIssuedAt}, {IssuedAt: fourthIssuedAt}}

	t.Run("EmptyRolesToken", func(t *testing.T) {
		p := AppProject{Spec: AppProjectSpec{}}
		needNormalize := p.NormalizeJWTTokens()
		assert.False(t, needNormalize)
		assert.Nil(t, p.Spec.Roles)
		assert.Nil(t, p.Status.JWTTokensByRole)
	})
	t.Run("HasRoles_NoTokens", func(t *testing.T) {
		p := AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role"}}}}
		needNormalize := p.NormalizeJWTTokens()
		assert.False(t, needNormalize)
	})
	t.Run("SpecRolesToken-StatusRolesTokenEmpty", func(t *testing.T) {
		p := AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", JWTTokens: testTokens}}}}
		needNormalize := p.NormalizeJWTTokens()
		assert.True(t, needNormalize)
		assert.ElementsMatch(t, p.Spec.Roles[0].JWTTokens, p.Status.JWTTokensByRole["test-role"].Items)
	})
	t.Run("SpecRolesEmpty-StatusRolesToken", func(t *testing.T) {
		p := AppProject{
			Spec:   AppProjectSpec{Roles: []ProjectRole{{Name: "test-role"}}},
			Status: AppProjectStatus{JWTTokensByRole: tokensByRole},
		}
		needNormalize := p.NormalizeJWTTokens()
		assert.True(t, needNormalize)
		assert.ElementsMatch(t, p.Spec.Roles[0].JWTTokens, p.Status.JWTTokensByRole["test-role"].Items)
	})
	t.Run("SpecRolesToken-StatusRolesToken-Same", func(t *testing.T) {
		p := AppProject{
			Spec:   AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", JWTTokens: testTokens}}},
			Status: AppProjectStatus{JWTTokensByRole: tokensByRole},
		}
		needNormalize := p.NormalizeJWTTokens()
		assert.False(t, needNormalize)
		assert.ElementsMatch(t, p.Spec.Roles[0].JWTTokens, p.Status.JWTTokensByRole["test-role"].Items)
	})
	t.Run("SpecRolesToken-StatusRolesToken-DifferentToken", func(t *testing.T) {
		p := AppProject{
			Spec:   AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", JWTTokens: testTokens2}}},
			Status: AppProjectStatus{JWTTokensByRole: tokensByRole},
		}
		needNormalize := p.NormalizeJWTTokens()
		assert.True(t, needNormalize)
		assert.ElementsMatch(t, p.Spec.Roles[0].JWTTokens, p.Status.JWTTokensByRole["test-role"].Items)
	})
	t.Run("SpecRolesToken-StatusRolesToken-DifferentRole", func(t *testing.T) {
		jwtTokens0 := []JWTToken{{IssuedAt: issuedAt}}
		jwtTokens1 := []JWTToken{{IssuedAt: issuedAt}, {IssuedAt: secondIssuedAt}}
		p := AppProject{
			Spec: AppProjectSpec{Roles: []ProjectRole{
				{Name: "test-role", JWTTokens: jwtTokens0},
				{Name: "test-role1", JWTTokens: jwtTokens1},
				{Name: "test-role2"},
			}},
			Status: AppProjectStatus{JWTTokensByRole: tokensByRole},
		}
		needNormalize := p.NormalizeJWTTokens()
		assert.True(t, needNormalize)
		assert.ElementsMatch(t, p.Spec.Roles[0].JWTTokens, p.Status.JWTTokensByRole["test-role"].Items)
		assert.ElementsMatch(t, p.Spec.Roles[1].JWTTokens, p.Status.JWTTokensByRole["test-role1"].Items)
		assert.ElementsMatch(t, p.Spec.Roles[2].JWTTokens, p.Status.JWTTokensByRole["test-role2"].Items)
	})
}

func TestRetryStrategy_NextRetryAtDefaultBackoff(t *testing.T) {
	retry := RetryStrategy{}
	now := time.Now()
	expectedTimes := map[int]time.Time{
		0:   now.Add(5 * time.Second),
		1:   now.Add(10 * time.Second),
		2:   now.Add(20 * time.Second),
		3:   now.Add(40 * time.Second),
		4:   now.Add(80 * time.Second),
		80:  now.Add(DefaultSyncRetryMaxDuration),
		100: now.Add(DefaultSyncRetryMaxDuration),
	}

	for i, expected := range expectedTimes {
		retryAt, err := retry.NextRetryAt(now, int64(i))
		require.NoError(t, err)
		assert.Equal(t, expected.Format(time.RFC850), retryAt.Format(time.RFC850))
	}
}

func TestRetryStrategy_NextRetryAtCustomBackoff(t *testing.T) {
	retry := RetryStrategy{
		Backoff: &Backoff{
			Duration:    "2s",
			Factor:      ptr.To(int64(3)),
			MaxDuration: "1m",
		},
	}
	now := time.Now()
	expectedTimes := []time.Time{
		now.Add(2 * time.Second),
		now.Add(6 * time.Second),
		now.Add(18 * time.Second),
		now.Add(54 * time.Second),
		now.Add(60 * time.Second),
	}

	for i, expected := range expectedTimes {
		retryAt, err := retry.NextRetryAt(now, int64(i))
		require.NoError(t, err)
		assert.Equal(t, expected.Format(time.RFC850), retryAt.Format(time.RFC850))
	}
}

func TestSourceAllowsConcurrentProcessing_KustomizeParams(t *testing.T) {
	t.Run("Has NameSuffix", func(t *testing.T) {
		src := ApplicationSource{Path: ".", Kustomize: &ApplicationSourceKustomize{
			NameSuffix: "test",
		}}

		assert.False(t, src.AllowsConcurrentProcessing())
	})

	t.Run("Has CommonAnnotations", func(t *testing.T) {
		src := ApplicationSource{Path: ".", Kustomize: &ApplicationSourceKustomize{
			CommonAnnotations: map[string]string{"foo": "bar"},
		}}

		assert.False(t, src.AllowsConcurrentProcessing())
	})

	t.Run("Has Patches", func(t *testing.T) {
		src := ApplicationSource{Path: ".", Kustomize: &ApplicationSourceKustomize{
			Patches: KustomizePatches{{
				Path: "test",
			}},
		}}

		assert.False(t, src.AllowsConcurrentProcessing())
	})
}

func TestUnSetCascadedDeletion(t *testing.T) {
	a := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Finalizers: []string{
				"alpha",
				ForegroundPropagationPolicyFinalizer,
				"beta",
				BackgroundPropagationPolicyFinalizer,
				"gamma",
			},
		},
	}
	a.UnSetCascadedDeletion()
	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, a.GetFinalizers())
}

func TestRemoveEnvEntry(t *testing.T) {
	t.Run("Remove element from the list", func(t *testing.T) {
		plugins := &ApplicationSourcePlugin{
			Name: "test",
			Env: Env{
				&EnvEntry{"foo", "bar"},
				&EnvEntry{"alpha", "beta"},
				&EnvEntry{"gamma", "delta"},
			},
		}
		require.NoError(t, plugins.RemoveEnvEntry("alpha"))
		want := Env{&EnvEntry{"foo", "bar"}, &EnvEntry{"gamma", "delta"}}
		assert.Equal(t, want, plugins.Env)
	})
	t.Run("Remove only element from the list", func(t *testing.T) {
		plugins := &ApplicationSourcePlugin{
			Name: "test",
			Env:  Env{&EnvEntry{"foo", "bar"}},
		}
		require.NoError(t, plugins.RemoveEnvEntry("foo"))
		assert.Equal(t, Env{}, plugins.Env)
	})
	t.Run("Remove unknown element from the list", func(t *testing.T) {
		plugins := &ApplicationSourcePlugin{
			Name: "test",
			Env:  Env{&EnvEntry{"foo", "bar"}},
		}
		err := plugins.RemoveEnvEntry("key")
		require.EqualError(t, err, `unable to find env variable with key "key" for plugin "test"`)
		err = plugins.RemoveEnvEntry("bar")
		require.EqualError(t, err, `unable to find env variable with key "bar" for plugin "test"`)
		assert.Equal(t, Env{&EnvEntry{"foo", "bar"}}, plugins.Env)
	})
	t.Run("Remove element from an empty list", func(t *testing.T) {
		plugins := &ApplicationSourcePlugin{Name: "test"}
		err := plugins.RemoveEnvEntry("key")
		require.EqualError(t, err, `unable to find env variable with key "key" for plugin "test"`)
	})
}

func TestOrphanedResourcesMonitorSettings_IsWarn(t *testing.T) {
	settings := OrphanedResourcesMonitorSettings{}
	assert.False(t, settings.IsWarn())

	settings.Warn = ptr.To(false)
	assert.False(t, settings.IsWarn())

	settings.Warn = ptr.To(true)
	assert.True(t, settings.IsWarn())
}

func Test_isValidPolicyObject(t *testing.T) {
	policyTests := []struct {
		name    string
		policy  string
		isValid bool
	}{
		{
			name:    "policy with full wildcard",
			policy:  "some-project/*",
			isValid: true,
		},
		{
			name:    "policy with specified project and application",
			policy:  "some-project/some-application",
			isValid: true,
		},
		{
			name:    "policy with full wildcard namespace and application",
			policy:  "some-project/*/*",
			isValid: true,
		},
		{
			name:    "policy with wildcard namespace and specified application",
			policy:  "some-project/*/some-application",
			isValid: true,
		},
		{
			name:    "policy with specified namespace and wildcard application",
			policy:  "some-project/some-namespace/*",
			isValid: true,
		},
		{
			name:    "policy with wildcard prefix namespace and specified application",
			policy:  "some-project/some-name*/some-application",
			isValid: true,
		},
		{
			name:    "policy with specified namespace and wildcard prefixed application",
			policy:  "some-project/some-namespace/some-app*",
			isValid: true,
		},
		{
			name:    "policy with valid namespace and application",
			policy:  "some-project/some-namespace/some-application",
			isValid: true,
		},
		{
			name:    "policy with invalid namespace character",
			policy:  "some-project/some~namespace/some-application",
			isValid: false,
		},
		{
			name:    "policy with invalid application character",
			policy:  "some-project/some-namespace/some^application",
			isValid: false,
		},
	}

	for _, policyTest := range policyTests {
		assert.Equal(t, policyTest.isValid, isValidObject("some-project", policyTest.policy), policyTest.name)
	}
}

func Test_validatePolicy_projIsNotRegex(t *testing.T) {
	// Make sure the "." in "some.project" isn't treated as the regex wildcard.
	err := validatePolicy("some.project", "org-admin", "p, proj:some.project:org-admin, applications, *, some-project/*, allow")
	require.Error(t, err)

	err = validatePolicy("some.project", "org-admin", "p, proj:some.project:org-admin, applications, *, some.project/*, allow")
	require.NoError(t, err)

	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, applications, *, some-project/*, allow")
	require.NoError(t, err)
}

func Test_validatePolicy_ValidResource(t *testing.T) {
	err := validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, applications, *, some-project/*, allow")
	require.NoError(t, err)
	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, repositories, *, some-project/*, allow")
	require.NoError(t, err)
	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, clusters, *, some-project/*, allow")
	require.NoError(t, err)
	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, exec, *, some-project/*, allow")
	require.NoError(t, err)
	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, logs, *, some-project/*, allow")
	require.NoError(t, err)
	err = validatePolicy("some-project", "org-admin", "p, proj:some-project:org-admin, unknown, *, some-project/*, allow")
	require.Error(t, err)
}

func TestEnvsubst(t *testing.T) {
	env := Env{
		&EnvEntry{"foo", "bar"},
	}

	assert.Equal(t, "bar", env.Envsubst("$foo"))
	assert.Equal(t, "$foo", env.Envsubst("$$foo"))
}

func Test_validateGroupName(t *testing.T) {
	tcs := []struct {
		name      string
		groupname string
		isvalid   bool
	}{
		{"Just a double quote", "\"", false},
		{"Just two double quotes", "\"\"", false},
		{"Normal group name", "foo", true},
		{"Quoted with commas", "\"foo,bar,baz\"", true},
		{"Quoted without commas", "\"foo\"", true},
		{"Quoted with leading and trailing whitespace", "  \"foo\" ", false},
		{"Empty group name", "", false},
		{"Empty group name with quotes", "\"\"", false},
		{"Unquoted with comma", "foo,bar,baz", false},
		{"Improperly quoted 1", "\"foo,bar,baz", false},
		{"Improperly quoted 2", "foo,bar,baz\"", false},
		{"Runaway quote in unqouted string", "foo,bar\",baz", false},
		{"Runaway quote in quoted string", "\"foo,\"bar,baz\"", false},
		{"Invalid characters unqouted", "foo\nbar", false},
		{"Invalid characters qouted", "\"foo\nbar\"", false},
		{"Runaway quote 1", "\"foo", false},
		{"Runaway quote 2", "foo\"", false},
	}
	for _, tt := range tcs {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroupName(tt.groupname)
			if tt.isvalid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGetCAPath(t *testing.T) {
	temppath := t.TempDir()
	cert, err := os.ReadFile("../../../../test/fixture/certs/argocd-test-server.crt")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(path.Join(temppath, "foo.example.com"), cert, 0o666)
	if err != nil {
		panic(err)
	}
	t.Setenv(argocdcommon.EnvVarTLSDataPath, temppath)
	validcert := []string{
		"https://foo.example.com",
		"oci://foo.example.com",
		"foo.example.com",
		"foo.example.com/charts",
		"https://foo.example.com:5000",
		"foo.example.com:5000",
		"foo.example.com:5000/charts",
		"ssh://foo.example.com",
	}
	invalidpath := []string{
		"https://bar.example.com",
		"oci://bar.example.com",
		"bar.example.com",
		"ssh://bar.example.com",
		"git@foo.example.com:organization/reponame.git",
		"ssh://git@foo.example.com:organization/reponame.git",
		"/some/invalid/thing",
		"../another/invalid/thing",
		"./also/invalid",
		"$invalid/as/well",
		"..",
		"://invalid",
	}

	for _, str := range validcert {
		path := getCAPath(str)
		assert.NotEmpty(t, path)
	}
	for _, str := range invalidpath {
		path := getCAPath(str)
		assert.Empty(t, path)
	}
}

func TestAppProjectIsSourceNamespacePermitted(t *testing.T) {
	app1 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app1",
			Namespace: "argocd",
		},
		Spec: ApplicationSpec{},
	}
	app2 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "some-ns",
		},
		Spec: ApplicationSpec{},
	}
	app3 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "",
		},
		Spec: ApplicationSpec{},
	}
	app4 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "other-ns",
		},
		Spec: ApplicationSpec{},
	}
	app5 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "some-ns1",
		},
		Spec: ApplicationSpec{},
	}
	app6 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "some-ns2",
		},
		Spec: ApplicationSpec{},
	}
	app7 := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app2",
			Namespace: "someotherns",
		},
		Spec: ApplicationSpec{},
	}
	t.Run("App in same namespace as controller", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: AppProjectSpec{
				SourceNamespaces: []string{"other-ns"},
			},
		}
		// app1 is installed to argocd namespace, controller as well
		assert.True(t, proj.IsAppNamespacePermitted(app1, "argocd"))
		// app2 is installed to some-ns namespace, controller as well
		assert.True(t, proj.IsAppNamespacePermitted(app2, "some-ns"))
		// app3 has no namespace set, so will be implicitly created in controller's namespace
		assert.True(t, proj.IsAppNamespacePermitted(app3, "argocd"))
	})
	t.Run("App not permitted when sourceNamespaces is empty", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: AppProjectSpec{
				SourceNamespaces: []string{},
			},
		}
		// app1 is installed to argocd namespace
		assert.True(t, proj.IsAppNamespacePermitted(app1, "argocd"))
		// app2 is installed to some-ns, controller running in argocd
		assert.False(t, proj.IsAppNamespacePermitted(app2, "argocd"))
	})

	t.Run("App permitted when sourceNamespaces has app namespace", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: AppProjectSpec{
				SourceNamespaces: []string{"some-ns"},
			},
		}
		// app2 is installed to some-ns, controller running in argocd
		assert.True(t, proj.IsAppNamespacePermitted(app2, "argocd"))
		// app4 is installed to other-ns, controller running in argocd
		assert.False(t, proj.IsAppNamespacePermitted(app4, "argocd"))
	})

	t.Run("App permitted by glob pattern", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: AppProjectSpec{
				SourceNamespaces: []string{"some-*"},
			},
		}
		// app5 is installed to some-ns1, controller running in argocd
		assert.True(t, proj.IsAppNamespacePermitted(app5, "argocd"))
		// app6 is installed to some-ns2, controller running in argocd
		assert.True(t, proj.IsAppNamespacePermitted(app6, "argocd"))
		// app7 is installed to someotherns, controller running in argocd
		assert.False(t, proj.IsAppNamespacePermitted(app7, "argocd"))
	})
}

func Test_RBACName(t *testing.T) {
	testApp := func(namespace, project string) *Application {
		return &Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: namespace,
			},
			Spec: ApplicationSpec{
				Project: project,
			},
		}
	}
	t.Run("App in same namespace as controller when ns is argocd", func(t *testing.T) {
		a := testApp("argocd", "default")
		assert.Equal(t, "default/test-app", a.RBACName("argocd"))
	})
	t.Run("App in same namespace as controller when ns is not argocd", func(t *testing.T) {
		a := testApp("some-ns", "default")
		assert.Equal(t, "default/test-app", a.RBACName("some-ns"))
	})
	t.Run("App in different namespace as controller when ns is argocd", func(t *testing.T) {
		a := testApp("some-ns", "default")
		assert.Equal(t, "default/some-ns/test-app", a.RBACName("argocd"))
	})
	t.Run("App in different namespace as controller when ns is not argocd", func(t *testing.T) {
		a := testApp("some-ns", "default")
		assert.Equal(t, "default/some-ns/test-app", a.RBACName("other-ns"))
	})
	t.Run("App in same namespace as controller when project is not yet set", func(t *testing.T) {
		a := testApp("argocd", "")
		assert.Equal(t, "default/test-app", a.RBACName("argocd"))
	})
	t.Run("App in same namespace as controller when ns is not yet set", func(t *testing.T) {
		a := testApp("", "")
		assert.Equal(t, "default/test-app", a.RBACName("argocd"))
	})
}

func TestGetSummary(t *testing.T) {
	tree := ApplicationTree{}
	app := newTestApp()

	summary := tree.GetSummary(app)
	assert.Empty(t, summary.ExternalURLs)

	const annotationName = argocdcommon.AnnotationKeyLinkPrefix + "/my-link"
	const url = "https://example.com"
	app.Annotations = make(map[string]string)
	app.Annotations[annotationName] = url

	summary = tree.GetSummary(app)
	assert.Len(t, summary.ExternalURLs, 1)
	assert.Equal(t, url, summary.ExternalURLs[0])
}

func TestApplicationSourcePluginParameters_Environ_string(t *testing.T) {
	params := ApplicationSourcePluginParameters{
		{
			Name:    "version",
			String_: ptr.To("1.2.3"),
		},
	}
	environ, err := params.Environ()
	require.NoError(t, err)
	assert.Len(t, environ, 2)
	assert.Contains(t, environ, "PARAM_VERSION=1.2.3")
	paramsJson, err := json.Marshal(params)
	require.NoError(t, err)
	assert.Contains(t, environ, fmt.Sprintf("ARGOCD_APP_PARAMETERS=%s", paramsJson))
}

func TestApplicationSourcePluginParameters_Environ_array(t *testing.T) {
	params := ApplicationSourcePluginParameters{
		{
			Name:          "dependencies",
			OptionalArray: &OptionalArray{Array: []string{"redis", "minio"}},
		},
	}
	environ, err := params.Environ()
	require.NoError(t, err)
	assert.Len(t, environ, 3)
	assert.Contains(t, environ, "PARAM_DEPENDENCIES_0=redis")
	assert.Contains(t, environ, "PARAM_DEPENDENCIES_1=minio")
	paramsJson, err := json.Marshal(params)
	require.NoError(t, err)
	assert.Contains(t, environ, fmt.Sprintf("ARGOCD_APP_PARAMETERS=%s", paramsJson))
}

func TestApplicationSourcePluginParameters_Environ_map(t *testing.T) {
	params := ApplicationSourcePluginParameters{
		{
			Name: "helm-parameters",
			OptionalMap: &OptionalMap{
				Map: map[string]string{
					"image.repo": "quay.io/argoproj/argo-cd",
					"image.tag":  "v2.4.0",
				},
			},
		},
	}
	environ, err := params.Environ()
	require.NoError(t, err)
	assert.Len(t, environ, 3)
	assert.Contains(t, environ, "PARAM_HELM_PARAMETERS_IMAGE_REPO=quay.io/argoproj/argo-cd")
	assert.Contains(t, environ, "PARAM_HELM_PARAMETERS_IMAGE_TAG=v2.4.0")
	paramsJson, err := json.Marshal(params)
	require.NoError(t, err)
	assert.Contains(t, environ, fmt.Sprintf("ARGOCD_APP_PARAMETERS=%s", paramsJson))
}

func TestApplicationSourcePluginParameters_Environ_all(t *testing.T) {
	// Technically there's no rule against specifying multiple types as values. It's up to the CMP how to handle them.
	// Name collisions can happen for the convenience env vars. When in doubt, CMP authors should use the JSON env var.
	params := ApplicationSourcePluginParameters{
		{
			Name:    "some-name",
			String_: ptr.To("1.2.3"),
			OptionalArray: &OptionalArray{
				Array: []string{"redis", "minio"},
			},
			OptionalMap: &OptionalMap{
				Map: map[string]string{
					"image.repo": "quay.io/argoproj/argo-cd",
					"image.tag":  "v2.4.0",
				},
			},
		},
	}
	environ, err := params.Environ()
	require.NoError(t, err)
	assert.Len(t, environ, 6)
	assert.Contains(t, environ, "PARAM_SOME_NAME=1.2.3")
	assert.Contains(t, environ, "PARAM_SOME_NAME_0=redis")
	assert.Contains(t, environ, "PARAM_SOME_NAME_1=minio")
	assert.Contains(t, environ, "PARAM_SOME_NAME_IMAGE_REPO=quay.io/argoproj/argo-cd")
	assert.Contains(t, environ, "PARAM_SOME_NAME_IMAGE_TAG=v2.4.0")
	paramsJson, err := json.Marshal(params)
	require.NoError(t, err)
	assert.Contains(t, environ, fmt.Sprintf("ARGOCD_APP_PARAMETERS=%s", paramsJson))
}

func getApplicationSpec() *ApplicationSpec {
	return &ApplicationSpec{
		Source: &ApplicationSource{
			Path: "source",
		}, Sources: ApplicationSources{
			{
				Path: "sources/source1",
			}, {
				Path: "sources/source2",
			},
		},
	}
}

func TestGetSource(t *testing.T) {
	tests := []struct {
		name           string
		hasSources     bool
		hasSource      bool
		appSpec        *ApplicationSpec
		expectedSource ApplicationSource
	}{
		{"GetSource with Source and Sources field present", true, true, getApplicationSpec(), ApplicationSource{Path: "sources/source1"}},
		{"GetSource with only Sources field", true, false, getApplicationSpec(), ApplicationSource{Path: "sources/source1"}},
		{"GetSource with only Source field", false, true, getApplicationSpec(), ApplicationSource{Path: "source"}},
		{"GetSource with no Source and Sources field", false, false, getApplicationSpec(), ApplicationSource{}},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			if !testCopy.hasSources {
				testCopy.appSpec.Sources = nil
			}
			if !testCopy.hasSource {
				testCopy.appSpec.Source = nil
			}
			source := testCopy.appSpec.GetSource()
			assert.Equal(t, testCopy.expectedSource, source)
		})
	}
}

func TestGetSources(t *testing.T) {
	tests := []struct {
		name            string
		hasSources      bool
		hasSource       bool
		appSpec         *ApplicationSpec
		expectedSources ApplicationSources
	}{
		{"GetSources with Source and Sources field present", true, true, getApplicationSpec(), ApplicationSources{
			{Path: "sources/source1"},
			{Path: "sources/source2"},
		}},
		{"GetSources with only Sources field", true, false, getApplicationSpec(), ApplicationSources{
			{Path: "sources/source1"},
			{Path: "sources/source2"},
		}},
		{"GetSources with only Source field", false, true, getApplicationSpec(), ApplicationSources{
			{Path: "source"},
		}},
		{"GetSources with no Source and Sources field", false, false, getApplicationSpec(), ApplicationSources{}},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			if !testCopy.hasSources {
				testCopy.appSpec.Sources = nil
			}
			if !testCopy.hasSource {
				testCopy.appSpec.Source = nil
			}
			sources := testCopy.appSpec.GetSources()
			assert.Equal(t, testCopy.expectedSources, sources)
		})
	}
}

func TestOptionalArrayEquality(t *testing.T) {
	// Demonstrate that the JSON unmarshalling of an empty array parameter is an OptionalArray with the array field set
	// to an empty array.
	presentButEmpty := `{"array":[]}`
	param := ApplicationSourcePluginParameter{}
	err := json.Unmarshal([]byte(presentButEmpty), &param)
	require.NoError(t, err)
	jsonPresentButEmpty := param.OptionalArray
	require.Equal(t, &OptionalArray{Array: []string{}}, jsonPresentButEmpty)

	// We won't simulate the protobuf unmarshalling of an empty array parameter. By experimentation, this is how it's
	// unmarshalled.
	protobufPresentButEmpty := &OptionalArray{Array: nil}

	tests := []struct {
		name     string
		a        *OptionalArray
		b        *OptionalArray
		expected bool
	}{
		{"nil and nil", nil, nil, true},
		{"nil and empty", nil, jsonPresentButEmpty, false},
		{"nil and empty-containing-nil", nil, protobufPresentButEmpty, false},
		{"empty-containing-empty and nil", jsonPresentButEmpty, nil, false},
		{"empty-containing-nil and nil", protobufPresentButEmpty, nil, false},
		{"empty-containing-empty and empty-containing-empty", jsonPresentButEmpty, jsonPresentButEmpty, true},
		{"empty-containing-empty and empty-containing-nil", jsonPresentButEmpty, protobufPresentButEmpty, true},
		{"empty-containing-nil and empty-containing-empty", protobufPresentButEmpty, jsonPresentButEmpty, true},
		{"empty-containing-nil and empty-containing-nil", protobufPresentButEmpty, protobufPresentButEmpty, true},
		{"empty-containing-empty and non-empty", jsonPresentButEmpty, &OptionalArray{Array: []string{"a"}}, false},
		{"non-empty and empty-containing-nil", &OptionalArray{Array: []string{"a"}}, jsonPresentButEmpty, false},
		{"non-empty and non-empty", &OptionalArray{Array: []string{"a"}}, &OptionalArray{Array: []string{"a"}}, true},
		{"non-empty and non-empty different", &OptionalArray{Array: []string{"a"}}, &OptionalArray{Array: []string{"b"}}, false},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCopy.expected, testCopy.a.Equals(testCopy.b))
		})
	}
}

func TestOptionalMapEquality(t *testing.T) {
	// Demonstrate that the JSON unmarshalling of an empty map parameter is an OptionalMap with the map field set
	// to an empty map.
	presentButEmpty := `{"map":{}}`
	param := ApplicationSourcePluginParameter{}
	err := json.Unmarshal([]byte(presentButEmpty), &param)
	require.NoError(t, err)
	jsonPresentButEmpty := param.OptionalMap
	require.Equal(t, &OptionalMap{Map: map[string]string{}}, jsonPresentButEmpty)

	// We won't simulate the protobuf unmarshalling of an empty map parameter. By experimentation, this is how it's
	// unmarshalled.
	protobufPresentButEmpty := &OptionalMap{Map: nil}

	tests := []struct {
		name     string
		a        *OptionalMap
		b        *OptionalMap
		expected bool
	}{
		{"nil and nil", nil, nil, true},
		{"nil and empty-containing-empty", nil, jsonPresentButEmpty, false},
		{"nil and empty-containing-nil", nil, protobufPresentButEmpty, false},
		{"empty-containing-empty and nil", jsonPresentButEmpty, nil, false},
		{"empty-containing-nil and nil", protobufPresentButEmpty, nil, false},
		{"empty-containing-empty and empty-containing-empty", jsonPresentButEmpty, jsonPresentButEmpty, true},
		{"empty-containing-empty and empty-containing-nil", jsonPresentButEmpty, protobufPresentButEmpty, true},
		{"empty-containing-empty and non-empty", jsonPresentButEmpty, &OptionalMap{Map: map[string]string{"a": "b"}}, false},
		{"empty-containing-nil and empty-containing-empty", protobufPresentButEmpty, jsonPresentButEmpty, true},
		{"empty-containing-nil and empty-containing-nil", protobufPresentButEmpty, protobufPresentButEmpty, true},
		{"non-empty and empty-containing-empty", &OptionalMap{Map: map[string]string{"a": "b"}}, jsonPresentButEmpty, false},
		{"non-empty and non-empty", &OptionalMap{Map: map[string]string{"a": "b"}}, &OptionalMap{Map: map[string]string{"a": "b"}}, true},
		{"non-empty and non-empty different", &OptionalMap{Map: map[string]string{"a": "b"}}, &OptionalMap{Map: map[string]string{"a": "c"}}, false},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCopy.expected, testCopy.a.Equals(testCopy.b))
		})
	}
}

func TestApplicationSpec_GetSourcePtrByIndex(t *testing.T) {
	testCases := []struct {
		name        string
		application ApplicationSpec
		sourceIndex int
		expected    *ApplicationSource
	}{
		{
			name: "HasMultipleSources_ReturnsFirstSource",
			application: ApplicationSpec{
				Sources: []ApplicationSource{
					{RepoURL: "https://github.com/argoproj/test1.git"},
					{RepoURL: "https://github.com/argoproj/test2.git"},
				},
			},
			sourceIndex: 0,
			expected:    &ApplicationSource{RepoURL: "https://github.com/argoproj/test1.git"},
		},
		{
			name: "HasMultipleSources_ReturnsSourceAtIndex",
			application: ApplicationSpec{
				Sources: []ApplicationSource{
					{RepoURL: "https://github.com/argoproj/test1.git"},
					{RepoURL: "https://github.com/argoproj/test2.git"},
				},
			},
			sourceIndex: 1,
			expected:    &ApplicationSource{RepoURL: "https://github.com/argoproj/test2.git"},
		},
		{
			name: "HasSingleSource_ReturnsSource",
			application: ApplicationSpec{
				Source: &ApplicationSource{RepoURL: "https://github.com/argoproj/test.git"},
			},
			sourceIndex: 0,
			expected:    &ApplicationSource{RepoURL: "https://github.com/argoproj/test.git"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.application.GetSourcePtrByIndex(tc.sourceIndex)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestApplicationTree_GetShards(t *testing.T) {
	tree := &ApplicationTree{
		Nodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "node 1"}}, {ResourceRef: ResourceRef{Name: "node 2"}}, {ResourceRef: ResourceRef{Name: "node 3"}},
		},
		OrphanedNodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "orph-node 1"}}, {ResourceRef: ResourceRef{Name: "orph-node 2"}}, {ResourceRef: ResourceRef{Name: "orph-node 3"}},
		},
		Hosts: []HostInfo{
			{Name: "host 1"}, {Name: "host 2"}, {Name: "host 3"},
		},
	}

	shards := tree.GetShards(2)
	require.Len(t, shards, 5)
	require.Equal(t, &ApplicationTree{
		ShardsCount: 5,
		Nodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "node 1"}}, {ResourceRef: ResourceRef{Name: "node 2"}},
		},
	}, shards[0])
	require.Equal(t, &ApplicationTree{
		Nodes:         []ResourceNode{{ResourceRef: ResourceRef{Name: "node 3"}}},
		OrphanedNodes: []ResourceNode{{ResourceRef: ResourceRef{Name: "orph-node 1"}}},
	}, shards[1])
	require.Equal(t, &ApplicationTree{
		OrphanedNodes: []ResourceNode{{ResourceRef: ResourceRef{Name: "orph-node 2"}}, {ResourceRef: ResourceRef{Name: "orph-node 3"}}},
	}, shards[2])
	require.Equal(t, &ApplicationTree{
		Hosts: []HostInfo{{Name: "host 1"}, {Name: "host 2"}},
	}, shards[3])
	require.Equal(t, &ApplicationTree{
		Hosts: []HostInfo{{Name: "host 3"}},
	}, shards[4])
}

func TestApplicationTree_Merge(t *testing.T) {
	tree := &ApplicationTree{}
	tree.Merge(&ApplicationTree{
		ShardsCount: 5,
		Nodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "node 1"}}, {ResourceRef: ResourceRef{Name: "node 2"}},
		},
	})
	tree.Merge(&ApplicationTree{
		Nodes:         []ResourceNode{{ResourceRef: ResourceRef{Name: "node 3"}}},
		OrphanedNodes: []ResourceNode{{ResourceRef: ResourceRef{Name: "orph-node 1"}}},
	})
	tree.Merge(&ApplicationTree{
		OrphanedNodes: []ResourceNode{{ResourceRef: ResourceRef{Name: "orph-node 2"}}, {ResourceRef: ResourceRef{Name: "orph-node 3"}}},
	})
	tree.Merge(&ApplicationTree{
		Hosts: []HostInfo{{Name: "host 1"}, {Name: "host 2"}},
	})
	tree.Merge(&ApplicationTree{
		Hosts: []HostInfo{{Name: "host 3"}},
	})
	require.Equal(t, &ApplicationTree{
		Nodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "node 1"}}, {ResourceRef: ResourceRef{Name: "node 2"}}, {ResourceRef: ResourceRef{Name: "node 3"}},
		},
		OrphanedNodes: []ResourceNode{
			{ResourceRef: ResourceRef{Name: "orph-node 1"}}, {ResourceRef: ResourceRef{Name: "orph-node 2"}}, {ResourceRef: ResourceRef{Name: "orph-node 3"}},
		},
		Hosts: []HostInfo{
			{Name: "host 1"}, {Name: "host 2"}, {Name: "host 3"},
		},
	}, tree)
}
