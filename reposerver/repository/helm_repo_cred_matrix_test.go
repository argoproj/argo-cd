package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// helmCredCase is one raw credential-template URL against one raw Helm
// dependency repository URL. Both go through getRepoCredential, the
// repo-server check that attaches stored Helm credentials. It does not
// normalize. expected is whether those credentials are attached today.
type helmCredCase struct {
	intent   string
	credURL  string
	credType string
	repoURL  string
	expected bool
}

func helmCredAttached(credURL, credType, repoURL string) bool {
	cred := &v1alpha1.RepoCreds{URL: credURL, Type: credType, Password: "secret"}
	got := getRepoCredential([]*v1alpha1.RepoCreds{cred}, repoURL)
	return got != nil
}

func TestHelmRepoCredMatrix(t *testing.T) {
	t.Parallel()

	cases := []helmCredCase{
		{
			intent:   "Helm credentials for this repository apply to a chart under that repository",
			credURL:  "https://legitimate-repo.com",
			repoURL:  "https://legitimate-repo.com/charts/app",
			expected: true,
		},
		{
			intent:   "An OCI credential for this registry applies to a repository under it",
			credURL:  "oci://registry.example.com",
			credType: "oci",
			repoURL:  "registry.example.com/team/chart",
			expected: true,
		},
		{
			// Type oci prepends oci:// before the prefix check, so a dependency URL
			// that already starts with oci:// does not match a credential that also does.
			intent:   "An OCI-typed credential whose URL starts with oci:// does not attach to an oci:// dependency URL",
			credURL:  "oci://registry.example.com",
			credType: "oci",
			repoURL:  "oci://registry.example.com/team/chart",
			expected: false,
		},
		{
			intent:   "A schemeless registry credential applies to a repository under that registry",
			credURL:  "registry.example.com",
			repoURL:  "oci://registry.example.com/team/chart",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			got := helmCredAttached(tc.credURL, tc.credType, tc.repoURL)
			assert.Equal(t, tc.expected, got, "intent: %s\ncredURL: %q\ncredType: %q\nrepoURL: %q", tc.intent, tc.credURL, tc.credType, tc.repoURL)
		})
	}
}
