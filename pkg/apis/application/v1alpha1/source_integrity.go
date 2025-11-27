package v1alpha1

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/git"
)

type SourceIntegrity struct {
	// Git - policies for git source verification
	// Mandatory field until there are alternatives
	Git *SourceIntegrityGit `json:"git" protobuf:"bytes,1,name=git"`
}

type SourceIntegrityGitFunc func(gitClient git.Client, unresolvedRevision string) (*SourceIntegrityCheckResult, error)

func (f SourceIntegrityGitFunc) Verify(gitClient git.Client, unresolvedRevision string) (*SourceIntegrityCheckResult, error) {
	if f == nil {
		return nil, nil
	}
	return f(gitClient, unresolvedRevision)
}

// ForGit evaluate if there are cheks for the git sources per given ApplicationSource and returns a function performing such check.
// If there are no cheks for git or the application source, this returns nil.
// This indirection is needed to detect the existence of relevant criteria without their actual execution.
func (si *SourceIntegrity) ForGit(repoURL string) SourceIntegrityGitFunc {
	if si == nil || si.Git == nil {
		return nil
	}

	policies := si.Git.findMatchingPolicies(repoURL)
	nPolicies := len(policies)
	if nPolicies == 0 {
		log.Infof("No policies found for repo URL: %s", repoURL)
		return nil
	}
	if nPolicies > 1 {
		log.Infof("Multiple (%d) policies found for repo URL: %s. Using the first matching one", nPolicies, repoURL)
	}
	return policies[0].ForGit(repoURL)
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

func (gp *SourceIntegrityGitPolicy) ForGit(repoURL string) SourceIntegrityGitFunc {
	if gp.GPG != nil {
		return gp.GPG.forGit(repoURL)
	}

	log.Warnf("No verification configured for SourceIntegrity policy for %v", gp.Repos)
	return nil
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

func (g *SourceIntegrityGitPolicyGPG) forGit(repoURL string) SourceIntegrityGitFunc {
	if !IsGPGEnabled() {
		log.Warnf("SourceIntegrity criteria for %s declared, but GPG verification is turned off by ARGOCD_GPG_ENABLED", repoURL)
		return nil
	}

	if g.Mode == SourceIntegrityGitPolicyGPGModeNone {
		// Declare passing check - we have validated based on a policy that happens to do nothing
		// TODO, think uf not to report as unchecked, instead
		return nil
	}

	return g.verify
}

// verify reports if the repository satisfies the criteria specified. It performs no checks when disabled through ARGOCD_GPG_ENABLED.
func (g *SourceIntegrityGitPolicyGPG) verify(gitClient git.Client, unresolvedRevision string) (result *SourceIntegrityCheckResult, err error) {
	const checkName = "GIT/GPG"

	if g.Mode == SourceIntegrityGitPolicyGPGModeNone {
		// Declare passing check - we have validated based on a policy that happens to do nothing
		return &SourceIntegrityCheckResult{
			Checks: []SourceIntegrityCheckResultItem{
				{checkName, []string{}},
			},
		}, nil
	}

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
	case SourceIntegrityGitPolicyGPGModeHead:
		// verify tag if on tag, latest revision otherwise
		if !verifyingTag {
			tagRevs, err := gitClient.LsSignatures(commitSHA, false)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, tagRevs...)
		}
	case SourceIntegrityGitPolicyGPGModeStrict:
		// verify history from the current commit
		deepRevs, err := gitClient.LsSignatures(commitSHA, true)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, deepRevs...)
	default:
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
