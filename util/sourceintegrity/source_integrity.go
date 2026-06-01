package sourceintegrity

import (
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"
)

// HasCriteria returns true when any source matches project policy for Git integrity (GPG, etc.),
// or when a traditional Helm source matches a Helm provenance policy (not OCI Helm registries)
func HasCriteria(si *v1alpha1.SourceIntegrity, sources ...v1alpha1.ApplicationSource) bool {
	if si == nil {
		return false
	}

	for _, source := range sources {
		if source.IsZero() {
			continue
		}
		if hasHelmProvenanceCriteriaForSource(si, source) {
			return true
		}
		if source.IsOCI() {
			continue
		}
		if !source.IsHelm() {
			if si.Git != nil && lookupGit(si, source.RepoURL) != nil {
				return true
			}
		}
	}

	return false
}

func repoMatches(urlGlob string, repoURL string) int {
	if strings.HasPrefix(urlGlob, "!") {
		if glob.Match(urlGlob[1:], repoURL) {
			return -1
		}
	} else {
		if glob.Match(urlGlob, repoURL) {
			return 1
		}
	}

	return 0
}

// repoURLMatchesPolicyGlobs returns true if repoURL is included by the globs (and not excluded).
// Used by both findMatchingGitPolicies and findMatchingHelmPolicies.
func repoURLMatchesPolicyGlobs(globs []string, repoURL string) bool {
	include := false
	for _, g := range globs {
		m := repoMatches(g, repoURL)
		if m == -1 {
			return false
		}
		if m == 1 {
			include = true
		}
	}
	return include
}

// msgUnallowedKey: message when verification is signed by a key not in the policy's allowed keys.
const msgUnallowedKey = "signed with unallowed key (key_id=%s)"

// isKeyInAllowedList returns true if signerKeyID (short or long form) is in the allowed keys list.
func isKeyInAllowedList(allowedKeys []string, signerKeyID string) bool {
	signerShort := signerKeyID
	if s, err := KeyID(signerKeyID); err == nil {
		signerShort = s
	}
	for _, k := range allowedKeys {
		allowedKey, err := KeyID(k)
		if err != nil {
			continue
		}
		if strings.EqualFold(allowedKey, signerShort) {
			return true
		}
	}
	return false
}

// IsGPGEnabled returns true if the GPG feature is enabled
func IsGPGEnabled() bool {
	if en := os.Getenv("ARGOCD_GPG_ENABLED"); strings.EqualFold(en, "false") || strings.EqualFold(en, "no") {
		return false
	}
	return true
}
