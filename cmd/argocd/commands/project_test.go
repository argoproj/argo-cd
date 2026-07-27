package commands

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
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

func dummySIProject(name string, si *v1alpha1.SourceIntegrity, sk []v1alpha1.SignatureKey) v1alpha1.AppProject { // nolint:staticcheck
	return v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.AppProjectSpec{
			Description:     "No description",
			SourceIntegrity: si,
			SignatureKeys:   sk,
		},
	}
}

func Test_projList_SignatureKeyWarnings(t *testing.T) {
	policy := &v1alpha1.SourceIntegrityGitPolicy{
		Repos: []v1alpha1.SourceIntegrityGitPolicyRepo{
			{
				URL: "*",
			},
		},
		GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{
			Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeHead,
			Keys: []string{"ABCD1234ABCD1234"},
		},
	}

	sourceIntegrity := &v1alpha1.SourceIntegrity{
		Git: &v1alpha1.SourceIntegrityGit{
			Policies: []*v1alpha1.SourceIntegrityGitPolicy{
				policy,
			},
		},
	}

	signatureKeys := []v1alpha1.SignatureKey{ // nolint:staticcheck
		{
			KeyID: "ABCD1234ABCD1234",
		},
	}

	getExpectedOutput := func(projectName string, description string, expectedSourceIntegrity string) string {
		header := "NAME\tDESCRIPTION\tDESTINATIONS\tSOURCES\tCLUSTER-RESOURCE-WHITELIST\tNAMESPACE-RESOURCE-BLACKLIST\tSOURCE-INTEGRITY\tORPHANED-RESOURCES\tDESTINATION-SERVICE-ACCOUNTS"
		content := fmt.Sprintf("%s\t%s\t<none>\t<none>\t<none>\t<none>\t%s\tdisabled\t<none>", projectName, description, expectedSourceIntegrity)
		return fmt.Sprintf("%s\n%s\n", header, content)
	}

	tests := []struct {
		name             string
		projects         []v1alpha1.AppProject
		expectedOutput   string
		expectedWarnings []string
	}{
		{
			name:             "SourceIntegrity is empty, no warnings",
			projects:         []v1alpha1.AppProject{dummySIProject("empty-si", nil, []v1alpha1.SignatureKey{})}, // nolint:staticcheck
			expectedOutput:   getExpectedOutput("empty-si", "No description", "<none>"),
			expectedWarnings: []string{},
		},
		{
			name:             "Project has Git SourceIntegrity, no warnings",
			projects:         []v1alpha1.AppProject{dummySIProject("git-si", sourceIntegrity, []v1alpha1.SignatureKey{})}, // nolint:staticcheck
			expectedOutput:   getExpectedOutput("git-si", "No description", "GIT/GPG"),
			expectedWarnings: []string{},
		},
		{
			name:           "Project has SignatureKeys, warning",
			projects:       []v1alpha1.AppProject{dummySIProject("signature-keys", nil, signatureKeys)},
			expectedOutput: getExpectedOutput("signature-keys", "No description", "GIT/GPG"), // SignatureKeys are effectively Git + GPG
			expectedWarnings: []string{
				"Warning: Project signature-keys uses deprecated SignatureKeys. Use SourceIntegrity instead.\n",
			},
		},
		{
			name:           "Project has both Git SourceIntegrity and SignatureKeys, warning",
			projects:       []v1alpha1.AppProject{dummySIProject("git-si", sourceIntegrity, signatureKeys)},
			expectedOutput: getExpectedOutput("git-si", "No description", "GIT/GPG"),
			expectedWarnings: []string{
				"Warning: Project git-si uses deprecated SignatureKeys. Use SourceIntegrity instead.\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := mockProjectClient(t)
			projects.On("List", mock.Anything, mock.Anything).Return(&v1alpha1.AppProjectList{Items: tt.projects}, nil)

			cmd := NewProjectListCommand(&argocdclient.ClientOptions{})
			out, errOut, err := runCmd(t, cmd)
			require.NoError(t, err)

			tabbedOut := regexp.MustCompile(" {2,}").ReplaceAllString(out, "\t")

			assert.Equal(t, tt.expectedOutput, tabbedOut)
			assert.Equal(t, strings.Join(tt.expectedWarnings, "\n"), errOut)
		})
	}
}
