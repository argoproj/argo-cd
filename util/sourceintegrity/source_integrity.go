package sourceintegrity

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/glob"
)

type SourceIntegrityGitFunc func(gitClient git.Client, unresolvedRevision string) (*v1alpha1.SourceIntegrityCheckResult, error)

// TODO refactor to interface

func (f SourceIntegrityGitFunc) Verify(gitClient git.Client, unresolvedRevision string) (*v1alpha1.SourceIntegrityCheckResult, error) {
	if f == nil {
		return nil, nil
	}
	return f(gitClient, unresolvedRevision)
}

var _gpgDisabledLoggedAlready bool

// ForGit evaluate if there are cheks for the git sources per given ApplicationSource and returns a function performing such check.
// If there are no cheks for git or the application source, this returns nil.
// This indirection is needed to detect the existence of relevant criteria without their actual execution.
func ForGit(si *v1alpha1.SourceIntegrity, repoURL string) SourceIntegrityGitFunc {
	if si == nil || si.Git == nil {
		return nil
	}

	policies := findMatchingPolicies(si.Git, repoURL)
	nPolicies := len(policies)
	if nPolicies == 0 {
		log.Infof("No git source integrity policies found for repo URL: %s", repoURL)
		return nil
	}
	if nPolicies > 1 {
		log.Infof("Multiple (%d) git source integrity policies found for repo URL: %s. Using the first matching one", nPolicies, repoURL)
	}

	policy := policies[0]
	if policy.GPG != nil {
		if policy.GPG.Mode == v1alpha1.SourceIntegrityGitPolicyGPGModeNone {
			// Declare missing check because there is no verification performed
			return nil
		}

		if !_gpgDisabledLoggedAlready && !IsGPGEnabled() {
			log.Warnf("SourceIntegrity criteria for git+gpg declared, but it is turned off by ARGOCD_GPG_ENABLED")
			_gpgDisabledLoggedAlready = true
			return nil
		}

		return func(gitClient git.Client, unresolvedRevision string) (*v1alpha1.SourceIntegrityCheckResult, error) {
			return verify(policy.GPG, gitClient, unresolvedRevision)
		}
	}

	log.Warnf("No verification configured for SourceIntegrity policy for %v", policy.Repos)
	return nil
}

func findMatchingPolicies(si *v1alpha1.SourceIntegrityGit, repoURL string) (policies []*v1alpha1.SourceIntegrityGitPolicy) {
	for _, p := range si.Policies {
		for _, r := range p.Repos {
			if r == "*" || glob.Match(r, repoURL) {
				policies = append(policies, p)
			}
		}
	}
	return policies
}

func verify(g *v1alpha1.SourceIntegrityGitPolicyGPG, gitClient git.Client, unresolvedRevision string) (result *v1alpha1.SourceIntegrityCheckResult, err error) {
	const checkName = "GIT/GPG"

	var revisions []git.RevisionSignatureInfo

	verifyingTag := gitClient.IsAnnotatedTag(unresolvedRevision)
	// If on tag, verify tag in both head and strict mode
	if verifyingTag {
		tagRev, err := gitClient.TagSignature(unresolvedRevision)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, *tagRev)
	}

	commitSHA, err := gitClient.CommitSHA()
	if err != nil {
		return nil, err
	}

	switch g.Mode {
	case v1alpha1.SourceIntegrityGitPolicyGPGModeHead:
		// verify tag if on tag, latest revision otherwise
		if !verifyingTag {
			tagRevs, err := gitClient.LsSignatures(commitSHA, false)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, tagRevs...)
		}
	case v1alpha1.SourceIntegrityGitPolicyGPGModeStrict:
		// verify history from the current commit
		deepRevs, err := gitClient.LsSignatures(commitSHA, true)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, deepRevs...)
	default:
		return nil, fmt.Errorf("unknown GPG mode %q configured for GIT source integrity", g.Mode)
	}

	var problems []string
	for _, signatureInfo := range revisions {
		// TODO: For deep verification, the list of commits/problems can be too long to present to user, or even too long to transfer
		// TODO: Keep only the most recent commit for every given GPG key, as that is what is actionable for an admin anyway.
		problem := gpgProblemMessage(g, signatureInfo)
		if problem != "" {
			problems = append(problems, problem)
		}
	}

	return &v1alpha1.SourceIntegrityCheckResult{Checks: []v1alpha1.SourceIntegrityCheckResultItem{{
		Name:     checkName,
		Problems: problems,
	}}}, nil
}

// gpgProblemMessage generates a message describing GPG verification issues for a specific revision signature and the configured policy.
// When an empty string is returned, it means there is no problem - the validation has passed.
func gpgProblemMessage(g *v1alpha1.SourceIntegrityGitPolicyGPG, signatureInfo git.RevisionSignatureInfo) string {
	if signatureInfo.VerificationResult != git.GPGVerificationResultGood {
		return fmt.Sprintf(
			"Failed verifying revision %s by '%s': %s (key_id=%s)",
			signatureInfo.Revision, signatureInfo.AuthorIdentity, signatureInfo.VerificationResult, signatureInfo.SignatureKeyID,
		)
	}

	for _, allowedKey := range g.Keys {
		allowedKey, err := KeyID(allowedKey)
		if err != nil {
			log.Error(err.Error())
		}
		if allowedKey == signatureInfo.SignatureKeyID {
			return ""
		}
	}

	return fmt.Sprintf(
		"Failed verifying revision %s by '%s': signed with unallowed key (key_id=%s)",
		signatureInfo.Revision, signatureInfo.AuthorIdentity, signatureInfo.SignatureKeyID,
	)
}

// IsGPGEnabled returns true if the GPG feature is enabled
func IsGPGEnabled() bool {
	if en := os.Getenv("ARGOCD_GPG_ENABLED"); strings.EqualFold(en, "false") || strings.EqualFold(en, "no") {
		return false
	}
	return true
}
