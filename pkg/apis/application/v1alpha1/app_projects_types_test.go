package v1alpha1

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
