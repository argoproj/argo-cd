package v1alpha1

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/git"
	log "github.com/sirupsen/logrus"
)

type SourceIntegrity struct {
	// Git - policies for git source verification
	// Mandatory field until there are alternatives
	Git *SourceIntegrityGit `json:"git" protobuf:"bytes,1,name=git"`
}

// VerifyGit revision against the declared integrity constraint.
// Returns nil if the source integrity check is nil - no check was requested.
func (si *SourceIntegrity) VerifyGit(gitClient git.Client, unresolvedRevision string) (*SourceIntegrityCheckResult, error) {
	if si == nil || si.Git == nil {
		return nil, nil
	}

	repoURL := gitClient.RepoURL()
	policies := si.Git.findMatchingPolicies(repoURL)
	nPolicies := len(policies)
	if nPolicies == 0 {
		// Use empty result to indicate there were 0 checks done
		return &SourceIntegrityCheckResult{}, nil
	}
	if nPolicies > 1 {
		log.Warnf("Multiple (%d) policies found for repo URL: %s. Using the first matching one", nPolicies, repoURL)
	}
	return policies[0].Verify(gitClient, unresolvedRevision)
}

type SourceIntegrityGit struct {
	Policies []*SourceIntegrityGitPolicy `json:"policies" protobuf:"bytes,1,name=policies"`
}

func (si *SourceIntegrityGit) findMatchingPolicies(repoURL string) (policies []*SourceIntegrityGitPolicy) {
	for _, p := range si.Policies {
		for _, r := range p.Repos {
			if globMatch(r, repoURL, false) {
				policies = append(policies, p)
			}
		}
	}
	return policies
}

type SourceIntegrityGitPolicy struct {
	Repos []string `json:"repos" protobuf:"bytes,1,name=repos"`
	// Mandatory field until there are alternatives
	GPG *SourceIntegrityGitPolicyGPG `json:"gpg" protobuf:"bytes,2,name=gpg"`
}

func (gp *SourceIntegrityGitPolicy) Verify(gitClient git.Client, unresolvedRevision string) (*SourceIntegrityCheckResult, error) {
	if gp.GPG != nil {
		return gp.GPG.Verify(gitClient, unresolvedRevision)
	}

	// Use empty result to indicate there were 0 checks done
	log.Warnf("No verification configured for SourceIntegrity policy for %v", gp.Repos)
	return &SourceIntegrityCheckResult{}, nil
}

type SourceIntegrityGitPolicyGPGMode string

var (
	SourceIntegrityGitPolicyGPGModeNone   SourceIntegrityGitPolicyGPGMode = "none"
	SourceIntegrityGitPolicyGPGModeHead   SourceIntegrityGitPolicyGPGMode = "head"
	SourceIntegrityGitPolicyGPGModeStrict SourceIntegrityGitPolicyGPGMode = "strict"
)

type SourceIntegrityGitPolicyGPG struct {
	Mode SourceIntegrityGitPolicyGPGMode `json:"mode" protobuf:"bytes,1,name=mode"`
	Keys []string                        `json:"keys" protobuf:"bytes,3,name=keys"`
}

// Verify reports if the repository satisfies the criteria specified. It performs to checks when disabled through ARGOCD_GPG_ENABLED.
// TODO unresolvedRevision is likely unneeded
func (g *SourceIntegrityGitPolicyGPG) Verify(gitClient git.Client, unresolvedRevision string) (result *SourceIntegrityCheckResult, err error) {
	if !IsGPGEnabled() {
		log.Warnf("SourceIntegrity criteria for %s declared, but GPG verification is turned off by ARGOCD_GPG_ENABLED", gitClient.RepoURL())
		// Use empty result to indicate there were 0 checks done
		return &SourceIntegrityCheckResult{}, nil
	}

	const checkName = "GIT/GPG"

	if g.Mode == SourceIntegrityGitPolicyGPGModeNone {
		// Declare passing check - we have validated based on a policy that happens to do nothing
		// TODO, think of not to report as unchecked
		return &SourceIntegrityCheckResult{
			Checks: []SourceIntegrityCheckResultItem{
				{checkName, []string{}},
			},
		}, nil
	}

	var revisions []git.RevisionSignatureInfo

	verifyingTag := gitClient.IsAnnotatedTag(unresolvedRevision)

	if g.Mode == SourceIntegrityGitPolicyGPGModeHead {
		// Verify tag if on tag, latest revision otherwise
		if verifyingTag {
			tagRev, err := gitClient.TagSignature(unresolvedRevision)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, *tagRev)
		} else {
			tagRevs, err := gitClient.LsSignatures(unresolvedRevision, false)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, tagRevs...)
		}
	} else if g.Mode == SourceIntegrityGitPolicyGPGModeStrict {
		// Verify tag if on tag
		if verifyingTag {
			tagRev, err := gitClient.TagSignature(unresolvedRevision)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, *tagRev)
		}

		// Verify history from the current commit
		commitSHA, err := gitClient.CommitSHA()
		if err != nil {
			return nil, err
		}

		deepRevs, err := gitClient.LsSignatures(commitSHA, true)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, deepRevs...)
	} else {
		panic("Unknown GPG mode " + g.Mode)
	}

	var problems []string
	for _, signatureInfo := range revisions {
		// TODO: For deep verification, the list of commits/problems can be too long to present to user, or even too long to transfer
		// TODO: Keep only the most recent commit for every given GPG key, as that is what is actionable for an admin anyway.
		problem := gpgProblemMessage(signatureInfo, g.Keys)
		if problem != "" {
			problems = append(problems, problem)
		}
	}

	return &SourceIntegrityCheckResult{Checks: []SourceIntegrityCheckResultItem{{
		Name:     checkName,
		Problems: problems,
	}}}, nil
}

func gpgProblemMessage(signatureInfo git.RevisionSignatureInfo, allowedKeys []string) string {
	if signatureInfo.VerificationResult != git.GPGVerificationResultGood {
		return fmt.Sprintf(
			"Failed verifying revision %s by '%s': %s (key_id=%s)",
			signatureInfo.Revision, signatureInfo.AuthorIdentity, signatureInfo.VerificationResult, signatureInfo.SignatureKeyID,
		)
	}

	if !slices.Contains(allowedKeys, signatureInfo.SignatureKeyID) {
		return fmt.Sprintf(
			"Revision %s by '%s': signed with disallowed key '%s'",
			signatureInfo.Revision, signatureInfo.AuthorIdentity, signatureInfo.SignatureKeyID,
		)
	}

	return ""
}

// TODO how to indicate partial integrity with multi-source apps? How to point to given source?

// SourceIntegrityCheckResult represents a conclusion of the SourceIntegrity evaluation.
// Each check performed on a source(es), holds a check item representing all checks performed.
type SourceIntegrityCheckResult struct {
	// Checks holds a list of checks performed, with their eventual problems. If a check is not specified here,
	// it means it was not performed.
	Checks []SourceIntegrityCheckResultItem `protobuf:"bytes,1,opt,name=checks"`
}

type SourceIntegrityCheckResultItem struct {
	// Name of the check that is human-understandable pointing out to the kind of verification performed.
	Name string `protobuf:"bytes,1,name=name"`
	// Problems is a list of messages explaining why the check failed. Empty list means success.
	Problems []string `protobuf:"bytes,2,name=problems"`
}

func (r *SourceIntegrityCheckResult) PassedChecks() (names []string) {
	names = []string{}
	for _, item := range r.Checks {
		if len(item.Problems) == 0 {
			names = append(names, item.Name)
		}
	}
	return names
}

// Error returns a string describing the integrity problems found, or "" if the sources are valid
func (r *SourceIntegrityCheckResult) Error() error {
	var errs []error
	for _, check := range r.Checks {
		for _, p := range check.Problems {
			errs = append(errs, fmt.Errorf("%s: %s", check.Name, p))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// IsValid reports if some of the performed checks had any problem
func (r *SourceIntegrityCheckResult) IsValid() bool {
	for _, item := range r.Checks {
		if len(item.Problems) > 0 {
			return false
		}
	}
	return true
}

// IsGPGEnabled returns true if the GPG feature is enabled
func IsGPGEnabled() bool {
	if en := os.Getenv("ARGOCD_GPG_ENABLED"); strings.EqualFold(en, "false") || strings.EqualFold(en, "no") {
		return false
	}
	return true
}
