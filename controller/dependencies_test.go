package controller

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/stevenle/topsort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// dependency is a helper function to create an ApplicationDependency struct
// with a label selector from labels.
func dependency(labels []map[string]string) *v1alpha1.ApplicationDependency {
	lselector := make([]*v1.LabelSelector, len(labels))
	if len(labels) > 0 {
		for i := range labels {
			lselector[i] = &v1.LabelSelector{
				MatchLabels: labels[i],
			}
		}
	}
	if len(lselector) == 0 {
		return &v1alpha1.ApplicationDependency{}
	}
	deps := make([]v1alpha1.ApplicationSelector, 0, len(labels))
	for i := range lselector {
		deps = append(deps, v1alpha1.ApplicationSelector{LabelSelector: lselector[i]})
	}
	return &v1alpha1.ApplicationDependency{Selectors: deps}
}

// appWithDependency is a helper function to create an application with the given
// set of labels and a given set of dependency definitions.
func appWithDependency(name, namespace, project string, labels map[string]string, dependencies []map[string]string) *v1alpha1.Application {
	app := &v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: project,
		},
	}
	if len(dependencies) > 0 {
		app.Spec.DependsOn = dependency(dependencies)
	}
	return app
}

func Test_buildDependencyGraph(t *testing.T) {
	t.Run("No dependencies", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
			},
			Spec: v1alpha1.ApplicationSpec{
				DependsOn: nil,
			},
		}
		data := fakeData{
			apps: []runtime.Object{
				appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"foo": "bar"}, nil),
				appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"foo": "bar"}, nil),
				&app,
			},
			applicationNamespaces: []string{test.FakeArgoCDNamespace},
		}
		c := newFakeController(&data, nil)
		apps := topsort.NewGraph()
		err := c.appStateManager.(*appStateManager).buildDependencyGraph(&app, apps)
		assert.NoError(t, err)
		assert.False(t, apps.ContainsNode("parent"))
		assert.False(t, apps.ContainsNode("app1"))
		assert.False(t, apps.ContainsNode("app2"))
	})

	t.Run("Flat dependencies", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}, {"name": "app2"}, {"name": "app3"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, nil),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, nil),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps := topsort.NewGraph()
		err := c.appStateManager.(*appStateManager).buildDependencyGraph(&app, apps)
		require.NoError(t, err)
		assert.True(t, apps.ContainsNode("app1"))
		assert.True(t, apps.ContainsNode("app2"))
		assert.True(t, apps.ContainsNode("app3"))
		assert.True(t, apps.ContainsNode("parent"))
	})

	t.Run("Chained dependencies", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}, {"name": "app2"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app2"}}),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps := topsort.NewGraph()
		err := c.appStateManager.(*appStateManager).buildDependencyGraph(&app, apps)
		require.NoError(t, err)
		assert.True(t, apps.ContainsNode("app1"))
		assert.True(t, apps.ContainsNode("app2"))
		assert.True(t, apps.ContainsNode("app3"))
		assert.True(t, apps.ContainsNode("parent"))
	})

	// The circular dependency test for getDependencies() makes sure that we
	// do not run into infinite recursion and all dependencies are properly
	// retrieved.
	t.Run("Circular dependency", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}, {"name": "parent"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app2"}}),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, []map[string]string{{"name": "parent"}}),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps := topsort.NewGraph()
		err := c.appStateManager.(*appStateManager).buildDependencyGraph(&app, apps)
		require.NoError(t, err)
		assert.True(t, apps.ContainsNode("app1"))
		assert.True(t, apps.ContainsNode("app2"))
		assert.True(t, apps.ContainsNode("app3"))
		assert.True(t, apps.ContainsNode("parent"))
	})
}

func Test_ResolveDependencies(t *testing.T) {
	t.Run("No dependencies", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "foo",
				Namespace: test.FakeArgoCDNamespace,
			},
			Spec: v1alpha1.ApplicationSpec{
				DependsOn: nil,
			},
		}
		data := fakeData{
			apps: []runtime.Object{
				appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"foo": "bar"}, nil),
				appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"foo": "bar"}, nil),
				&app,
			},
			applicationNamespaces: []string{test.FakeArgoCDNamespace},
		}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		assert.NoError(t, err)
		assert.Len(t, apps, 0)
	})

	t.Run("Flat dependencies with label selector", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}, {"name": "app2"}, {"name": "app3"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, nil),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, nil),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 3)
		// Flat dependencies may be in any particular order
		require.Contains(t, apps, "app1")
		require.Contains(t, apps, "app2")
		require.Contains(t, apps, "app3")
	})

	t.Run("Flat dependencies with name selector", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				DependsOn: &v1alpha1.ApplicationDependency{
					Selectors: []v1alpha1.ApplicationSelector{
						{NamePattern: []string{"*1", "*2"}},
					},
				},
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, nil),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, nil),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 2)
		// Flat dependencies may be in any particular order
		require.Contains(t, apps, "app1")
		require.Contains(t, apps, "app2")
	})

	t.Run("Flat dependencies with mixed selector", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				DependsOn: &v1alpha1.ApplicationDependency{
					Selectors: []v1alpha1.ApplicationSelector{
						{
							LabelSelector: &v1.LabelSelector{
								MatchExpressions: []v1.LabelSelectorRequirement{
									{
										Key:      "name",
										Operator: v1.LabelSelectorOpExists,
									},
								},
							},
							NamePattern: []string{"app2", "app3"},
						},
					},
				},
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, nil),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, nil),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 2)
		// Flat dependencies may be in any particular order
		require.Contains(t, apps, "app2")
		require.Contains(t, apps, "app3")
	})

	t.Run("Flat dependencies with two selectors", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				DependsOn: &v1alpha1.ApplicationDependency{
					// These selectors should select app1 by name, and app3 by label
					Selectors: []v1alpha1.ApplicationSelector{
						{
							LabelSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{
									"name": "app3",
								},
							}},
						{
							NamePattern: []string{"app1"},
						},
					},
				},
			},
		}

		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", nil, nil),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", nil, nil),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 2)
		// Flat dependencies may be in any particular order
		require.Contains(t, apps, "app1")
		require.Contains(t, apps, "app3")
	})

	t.Run("Chained dependencies", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}, {"name": "app2"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app2"}}),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, nil),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 3)
		require.Equal(t, []string{"app3", "app2", "app1"}, apps)
	})

	t.Run("Circular chained dependency", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app2", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app2"}, []map[string]string{{"name": "app4"}}),
			appWithDependency("app3", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app3"}, []map[string]string{{"name": "parent"}}),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.ErrorContains(t, err, "Cycle error")
		require.Len(t, apps, 0)
	})

	// App depending on itself is a circular depedency which must result in an
	// error.
	t.Run("Parent depends on self", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "parent"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.ErrorContains(t, err, "Cycle error")
		require.Len(t, apps, 0)
	})

	t.Run("Dependency depends on self", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "parent"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.ErrorContains(t, err, "Cycle error")
		require.Len(t, apps, 0)
	})

	t.Run("Ignore apps using different AppProject", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}}),
			},
		}
		data := fakeData{apps: []runtime.Object{
			appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app2", test.FakeArgoCDNamespace, "other", map[string]string{"name": "app2"}, []map[string]string{{"name": "app3"}}),
			appWithDependency("app3", test.FakeArgoCDNamespace, "other", map[string]string{"name": "app3"}, []map[string]string{{"name": "parent"}}),
			&app,
		}}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 1)
	})

	t.Run("Ignore apps in different namespaces", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Name:      "parent",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"name": "parent",
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project:   "default",
				DependsOn: dependency([]map[string]string{{"name": "app1"}}),
			},
		}
		data := fakeData{
			apps: []runtime.Object{
				appWithDependency("app1", test.FakeArgoCDNamespace, "default", map[string]string{"name": "app1"}, []map[string]string{{"name": "app3"}}),
				appWithDependency("app2", "default", "default", map[string]string{"name": "app2"}, []map[string]string{{"name": "app3"}}),
				appWithDependency("app3", "default", "default", map[string]string{"name": "app3"}, []map[string]string{{"name": "parent"}}),
				&app,
			},
			applicationNamespaces: []string{"*"},
		}
		c := newFakeController(&data, nil)
		apps, err := c.appStateManager.(*appStateManager).ResolveApplicationDependencies(&app)
		require.NoError(t, err)
		require.Len(t, apps, 1)
	})

}
