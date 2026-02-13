package v1alpha1

import (
	"errors"
	"fmt"
)

type SourceIntegrity struct {
	// Git - policies for git source verification
	Git *SourceIntegrityGit `json:"git" protobuf:"bytes,1,name=git"` // A mandatory field until there are alternatives
}

type SourceIntegrityGit struct {
	Policies []*SourceIntegrityGitPolicy `json:"policies" protobuf:"bytes,1,name=policies"`
}

type SourceIntegrityGitPolicy struct {
	// List of repository glob patterns restricting repositories the policy will apply to
	Repos []string `json:"repos" protobuf:"bytes,1,name=repos"`
	// Verify GPG commit/tag signatures
	GPG *SourceIntegrityGitPolicyGPG `json:"gpg" protobuf:"bytes,2,name=gpg"` // A mandatory field until there are alternatives
}

type SourceIntegrityGitPolicyGPGMode string

var (
	// SourceIntegrityGitPolicyGPGModeNone performs no verification at all. This is useful to declare exceptions in more
	// general policies declared later (i.e.: verify in repositories in an organization, except for one).
	SourceIntegrityGitPolicyGPGModeNone SourceIntegrityGitPolicyGPGMode = "none"
	// SourceIntegrityGitPolicyGPGModeHead verifies the current target revision, an annotated tag or a commit.
	SourceIntegrityGitPolicyGPGModeHead SourceIntegrityGitPolicyGPGMode = "head"
	// SourceIntegrityGitPolicyGPGModeStrict verifies all ancestry of target revision all the way to git init or a seal commits.
	// If pointing to an annotated tag, it verified both the tag signature and the commit one.
	SourceIntegrityGitPolicyGPGModeStrict SourceIntegrityGitPolicyGPGMode = "strict"
)

// SourceIntegrityGitPolicyGPG verifies that the commit(s) are both correctly signed by a key in the repo-server keyring,
// and that they are signed by one of the key listed in Keys.
//
// This policy can be deactivated through the ARGOCD_GPG_ENABLED environment variable.
//
// Note the listing of problematic commits/signatures reported when "strict" mode validation fails may not be complete.
// This means that a user that has addressed all problems reported by source integrity check can run into
// further problematic signatures on a subsequent attempt. That happens namely when history contains seal commits signed
// with gpg keys that are in the keyring, but not listed in Keys.
type SourceIntegrityGitPolicyGPG struct {
	Mode SourceIntegrityGitPolicyGPGMode `json:"mode" protobuf:"bytes,1,name=mode"`
	// List of key IDs to trust. The keys need to be in the repository server keyring.
	Keys []string `json:"keys" protobuf:"bytes,3,name=keys"`
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

func (r *SourceIntegrityCheckResult) AsError() error {
	if r == nil {
		return nil
	}
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
