package commands

import (
	"bytes"
	"io"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	projectpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	projectmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/project/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestModifyResourceListCmd_AddClusterAllowItemWithName(t *testing.T) {
	// Create a mock project client
	mockProjClient := projectmocks.NewProjectServiceClient(t)

	// Mock project data
	projectName := "test-project"
	mockProject := &v1alpha1.AppProject{
		Spec: v1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []v1alpha1.ClusterResourceRestrictionItem{},
		},
	}

	// Mock Get and Update calls
	mockProjClient.On("Get", mock.Anything, mock.Anything).Return(mockProject, nil)
	mockProjClient.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*projectpkg.ProjectUpdateRequest)
		mockProject.Spec.ClusterResourceWhitelist = req.Project.Spec.ClusterResourceWhitelist
	}).Return(mockProject, nil)

	getProjIf := func(_ *cobra.Command) (io.Closer, projectpkg.ProjectServiceClient) {
		return io.NopCloser(bytes.NewBufferString("")), mockProjClient
	}
	// Create the command
	cmd := modifyResourceListCmd(
		getProjIf,
		"allow-cluster-resource",
		"Test command",
		"Example usage",
		true,
		false,
	)

	// Set up the command arguments
	args := []string{projectName, "apps", "Deployment", "example-deployment"}
	cmd.SetArgs(args)

	// Capture the output
	var output bytes.Buffer
	cmd.SetOut(&output)

	// Execute the command
	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	// Verify the project was updated correctly
	expected := []v1alpha1.ClusterResourceRestrictionItem{
		{Group: "apps", Kind: "Deployment", Name: "example-deployment"},
	}
	assert.Equal(t, expected, mockProject.Spec.ClusterResourceWhitelist)

	// Verify the output
	assert.Contains(t, output.String(), "Group 'apps', kind 'Deployment', and name 'example-deployment' is added to allowed cluster resources")
}

func Test_modifyNamespacedResourceList(t *testing.T) {
	tests := []struct {
		name           string
		initialList    []metav1.GroupKind
		add            bool
		group          string
		kind           string
		expectedList   []metav1.GroupKind
		expectedResult bool
	}{
		{
			name:        "Add new item to empty list",
			initialList: []metav1.GroupKind{},
			add:         true,
			group:       "apps",
			kind:        "Deployment",
			expectedList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			expectedResult: true,
		},
		{
			name: "Add duplicate item",
			initialList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			add:   true,
			group: "apps",
			kind:  "Deployment",
			expectedList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			expectedResult: false,
		},
		{
			name: "Remove existing item",
			initialList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			add:            false,
			group:          "apps",
			kind:           "Deployment",
			expectedList:   []metav1.GroupKind{},
			expectedResult: true,
		},
		{
			name: "Remove non-existent item",
			initialList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			add:   false,
			group: "apps",
			kind:  "StatefulSet",
			expectedList: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := tt.initialList
			result, _ := modifyNamespacedResourcesList(&list, tt.add, "", tt.group, tt.kind)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedList, list)
		})
	}
}

func Test_modifyAllowClusterResourceList(t *testing.T) {
	tests := []struct {
		name           string
		initialList    []v1alpha1.ClusterResourceRestrictionItem
		add            bool
		group          string
		kind           string
		resourceName   string
		expectedList   []v1alpha1.ClusterResourceRestrictionItem
		expectedResult bool
	}{
		{
			name:         "Add new item to empty list",
			initialList:  []v1alpha1.ClusterResourceRestrictionItem{},
			add:          true,
			group:        "apps",
			kind:         "Deployment",
			resourceName: "",
			expectedList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			expectedResult: true,
		},
		{
			name: "Add duplicate item",
			initialList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			add:          true,
			group:        "apps",
			kind:         "Deployment",
			resourceName: "",
			expectedList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			expectedResult: false,
		},
		{
			name: "Remove existing item",
			initialList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			add:            false,
			group:          "apps",
			kind:           "Deployment",
			resourceName:   "",
			expectedList:   []v1alpha1.ClusterResourceRestrictionItem{},
			expectedResult: true,
		},
		{
			name: "Remove non-existent item",
			initialList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			add:          false,
			group:        "apps",
			kind:         "StatefulSet",
			resourceName: "",
			expectedList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			expectedResult: false,
		},
		{
			name:         "Add item with name",
			initialList:  []v1alpha1.ClusterResourceRestrictionItem{},
			add:          true,
			group:        "apps",
			kind:         "Deployment",
			resourceName: "example-deployment",
			expectedList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: "example-deployment"},
			},
			expectedResult: true,
		},
		{
			name: "Remove item with name",
			initialList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: "example-deployment"},
			},
			add:            false,
			group:          "apps",
			kind:           "Deployment",
			resourceName:   "example-deployment",
			expectedList:   []v1alpha1.ClusterResourceRestrictionItem{},
			expectedResult: true,
		},
		{
			name: "Attempt to remove item with name but only group and kind exist",
			initialList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			add:          false,
			group:        "apps",
			kind:         "Deployment",
			resourceName: "example-deployment",
			expectedList: []v1alpha1.ClusterResourceRestrictionItem{
				{Group: "apps", Kind: "Deployment", Name: ""},
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := tt.initialList

			result, _ := modifyClusterResourcesList(&list, tt.add, "", tt.group, tt.kind, tt.resourceName)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedList, list)
		})
	}
}
