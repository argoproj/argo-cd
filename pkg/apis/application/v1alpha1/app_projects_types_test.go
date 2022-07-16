package v1alpha1

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_checkParents(t *testing.T) {
	t.Run("no parent, nothing to do", func(t *testing.T) {
		isPermitted, err := checkParents(
			&AppProject{},
			func(name string) (*AppProject, error) {
				t.Fatal("checkParents tried to get a project, which it shouldn't do, because the root project has no parents")
				return nil, nil
			},
			func(project *AppProject) (bool, error) {
				t.Fatal("checkParents tried to check a project, which it shouldn't do, because the root project has no parents, and the root project is not checked by checkParents")
				return false, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
	t.Run("one parent, no errors", func(t *testing.T) {
		testCases := []bool{false, true}

		for _, testCase := range testCases {
			testCase := testCase

			t.Run(fmt.Sprintf("permitted %v", testCase), func(t *testing.T) {
				isPermitted, err := checkParents(
					&AppProject{Spec: AppProjectSpec{ParentProject: "parent"}},
					func(name string) (*AppProject, error) {
						return &AppProject{}, nil
					},
					func(project *AppProject) (bool, error) {
						return testCase, nil
					},
				)
				assert.NoError(t, err)
				assert.Equal(t, testCase, isPermitted)
			})
		}
	})
	t.Run("one parent, error getting parent", func(t *testing.T) {
		expectedError := errors.New("failed to get parent project")
		isPermitted, err := checkParents(
			&AppProject{Spec: AppProjectSpec{ParentProject: "parent"}},
			func(name string) (*AppProject, error) {
				return nil, expectedError
			},
			func(project *AppProject) (bool, error) {
				return true, nil
			},
		)
		assert.ErrorIs(t, err, expectedError)
		assert.False(t, isPermitted)
	})
	t.Run("one parent, error checking", func(t *testing.T) {
		expectedError := errors.New("failed to check")
		isPermitted, err := checkParents(
			&AppProject{Spec: AppProjectSpec{ParentProject: "parent"}},
			func(name string) (*AppProject, error) {
				return &AppProject{}, nil
			},
			func(project *AppProject) (bool, error) {
				return false, expectedError
			},
		)
		assert.ErrorIs(t, err, expectedError)
		assert.False(t, isPermitted)
	})
	t.Run("loop", func(t *testing.T) {
		isPermitted, err := checkParents(
			&AppProject{ObjectMeta: v1.ObjectMeta{Name: "a"}, Spec: AppProjectSpec{ParentProject: "b"}},
			func(name string) (*AppProject, error) {
				if name == "a" {
					t.Fatal("checkProject looped back and checked the initial project - it should not do that")
				} else if name == "b" {
					return &AppProject{ObjectMeta: v1.ObjectMeta{Name: "b"}, Spec: AppProjectSpec{ParentProject: "a"}}, nil
				} else {
					t.Fatalf("checkProject tried to get project %q that wasn't referenced", name)
				}
				return nil, nil  // this shouldn't happen
			},
			func(project *AppProject) (bool, error) {
				return true, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
	t.Run("self-referential", func(t *testing.T) {
		isPermitted, err := checkParents(
			&AppProject{ObjectMeta: v1.ObjectMeta{Name: "root"}, Spec: AppProjectSpec{ParentProject: "root"}},
			func(name string) (*AppProject, error) {
				t.Fatal("checkParents tried to get a project, which it shouldn't do, because the root project has no parents")
				return nil, nil
			},
			func(project *AppProject) (bool, error) {
				t.Fatal("checkParents tried to check a project, which it shouldn't do, because the root project has no parents, and the root project is not checked by checkParents")
				return false, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
}

func Test_IsGroupKindPermitted(t *testing.T) {
	t.Run("parent projects block resources when child does not", func(t *testing.T) {
		projects := map[string]AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					ParentProject: "b",
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					ParentProject: "c",
					NamespaceResourceBlacklist: []v1.GroupKind{
						{Group: "v1", Kind: "ConfigMap"},
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					NamespaceResourceBlacklist: []v1.GroupKind{
						{Group: "v1", Kind: "ServiceAccount"},
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project := projects[name]
			return &project, nil
		}

		isPermitted, err := projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "ConfigMap"}, true, getProject)
		assert.NoError(t, err)
		assert.False(t, isPermitted)

		isPermitted, err = projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "ServiceAccount"}, true, getProject)
		assert.NoError(t, err)
		assert.False(t, isPermitted)

		isPermitted, err = projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "Pod"}, true, getProject)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
}

func Test_IsDestinationPermitted(t *testing.T) {
	t.Run("parent projects block destination when child does not", func(t *testing.T) {
		projects := map[string]AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					ParentProject: "b",
					Destinations: []ApplicationDestination{
						{Server: "*", Name: "*", Namespace: "*"},
					},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					ParentProject: "c",
					Destinations: []ApplicationDestination{
						{Name: "dev-usw2-*", Namespace: "dev-usw2-*"},
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "dev-*", Namespace: "dev-*"},
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project := projects[name]
			return &project, nil
		}

		isPermitted, err := projects["root"].IsDestinationPermitted(ApplicationDestination{Name: "dev-use2-cluster", Namespace: "dev-use2-namespace"}, getProject)
		assert.NoError(t, err)
		assert.False(t, isPermitted)

		isPermitted, err = projects["root"].IsDestinationPermitted(ApplicationDestination{Name: "dev-usw2-cluster", Namespace: "dev-usw2-namespace"}, getProject)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
}

func Test_IsSourcePermitted(t *testing.T) {
	t.Run("parent projects block source when child does not", func(t *testing.T) {
		projects := map[string]AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					ParentProject: "b",
					SourceRepos: []string{
						"https://github.company.com/dev-org/*",
					},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					ParentProject: "c",
					SourceRepos: []string{
						"https://github.company.com/dev-org/*-deployment.git",
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					SourceRepos: []string{
						"https://github.company.com/*/*",
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project := projects[name]
			return &project, nil
		}

		isPermitted, err := projects["root"].IsSourcePermitted(ApplicationSource{RepoURL: "https://github.company.com/dev-org/app1-deployment.git"}, getProject)
		assert.NoError(t, err)
		assert.True(t, isPermitted)

		isPermitted, err = projects["root"].IsSourcePermitted(ApplicationSource{RepoURL: "https://github.company.com/other-org/app1-deployment.git"}, getProject)
		assert.NoError(t, err)
		assert.False(t, isPermitted)
	})
}

