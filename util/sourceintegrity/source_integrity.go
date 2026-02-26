package sourceintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/glob"
)

type gitFunc func(gitClient git.Client, verifiedRevision string) (*v1alpha1.SourceIntegrityCheckResult, error)

var _gpgDisabledLoggedAlready bool

// HasCriteria determines if any of the sources have some criteria declared
func HasCriteria(si *v1alpha1.SourceIntegrity, sources ...v1alpha1.ApplicationSource) bool {
	if si == nil {
		return false
	}

	for _, source := range sources {
		if source.IsZero() || source.IsOCI() {
			continue
		}
		if !source.IsHelm() {
			if si.Git != nil && lookupGit(si, source.RepoURL) != nil {
				return true
			}
		} else if hasHelmProvenanceCriteriaForSource(si, source) {
			return true
		}
	}

	return false
}

// HasHelmProvenanceCriteria returns true only when at least one source is Helm and the project
// has a matching Helm provenance policy (mode != none).
func HasHelmProvenanceCriteria(si *v1alpha1.SourceIntegrity, sources ...v1alpha1.ApplicationSource) bool {
	if si == nil {
		return false
	}
	for _, source := range sources {
		if source.IsZero() || source.IsOCI() {
			continue
		}
		if hasHelmProvenanceCriteriaForSource(si, source) {
			return true
		}
	}
	return false
}

func hasHelmProvenanceCriteriaForSource(si *v1alpha1.SourceIntegrity, source v1alpha1.ApplicationSource) bool {
	if !source.IsHelm() || si == nil || si.Helm == nil {
		return false
	}
	policies := findMatchingHelmPolicies(si.Helm, source.RepoURL)
	for _, p := range policies {
		if p.Provenance != nil && p.Provenance.Mode == v1alpha1.SourceIntegrityHelmPolicyProvenanceModeProvenance {
			return true
		}
	}
	return false
}

// VerifyGit makes sure the git repository satisfies the criteria declared.
// It returns nil in case there were no relevant criteria, a check result if there were.
// The verifiedRevision is expected to be either an annotated tag to a resolved commit sha - the revision, its signature is being verified.
func VerifyGit(si *v1alpha1.SourceIntegrity, gitClient git.Client, verifiedRevision string) (result *v1alpha1.SourceIntegrityCheckResult, err error) {
	if si == nil || si.Git == nil {
		return nil, nil
	}

	check := lookupGit(si, gitClient.RepoURL())
	if check != nil {
		return check(gitClient, verifiedRevision)
	}
	return nil, nil
}

func lookupGit(si *v1alpha1.SourceIntegrity, repoURL string) gitFunc {
	policies := findMatchingGitPolicies(si.Git, repoURL)
	nPolicies := len(policies)
	if nPolicies == 0 {
		log.Infof("No git source integrity policies found for repo URL: %s", repoURL)
		return nil
	}
	if nPolicies > 1 {
		// Multiple matching policies is an error. BUT, it has to return a check that fails for every repo.
		// This is to make sure that a mistake in argo cd configuration does not disable verification until fixed.
		msg := fmt.Sprintf("multiple (%d) git source integrity policies found for repo URL: %s", nPolicies, repoURL)
		log.Warn(msg)
		return func(_ git.Client, _ string) (*v1alpha1.SourceIntegrityCheckResult, error) {
			return nil, errors.New(msg)
		}
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

		return func(gitClient git.Client, verifiedRevision string) (*v1alpha1.SourceIntegrityCheckResult, error) {
			return verify(policy.GPG, gitClient, verifiedRevision)
		}
	}

	log.Warnf("No verification configured for SourceIntegrity policy for %+v", policy.Repos)
	return nil
}

func findMatchingGitPolicies(si *v1alpha1.SourceIntegrityGit, repoURL string) (policies []*v1alpha1.SourceIntegrityGitPolicy) {
	for _, p := range si.Policies {
		globs := make([]string, 0, len(p.Repos))
		for _, r := range p.Repos {
			globs = append(globs, r.URL)
		}
		if repoURLMatchesPolicyGlobs(globs, repoURL) {
			policies = append(policies, p)
		}
	}
	return policies
}

func repoMatches(urlGlob string, repoURL string) int {
	if strings.HasPrefix(urlGlob, "!") {
		inner := urlGlob[1:]
		matched := glob.Match(inner, repoURL)
		if matched {
			return -1
		}
	} else {
		matched := glob.Match(urlGlob, repoURL)
		if matched {
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

func verify(g *v1alpha1.SourceIntegrityGitPolicyGPG, gitClient git.Client, verifiedRevision string) (result *v1alpha1.SourceIntegrityCheckResult, err error) {
	const checkName = "GIT/GPG"

	var deep bool
	switch g.Mode {
	// verify tag if on tag, latest revision otherwise
	case v1alpha1.SourceIntegrityGitPolicyGPGModeHead:
		deep = false
	// verify history from the current commit
	case v1alpha1.SourceIntegrityGitPolicyGPGModeStrict:
		deep = true
	default:
		return nil, fmt.Errorf("unknown GPG mode %q configured for GIT source integrity", g.Mode)
	}

	signatures, err := gitClient.LsSignatures(verifiedRevision, deep)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.SourceIntegrityCheckResult{Checks: []v1alpha1.SourceIntegrityCheckResultItem{{
		Name:     checkName,
		Problems: describeProblems(g, signatures),
	}}}, nil
}

// describeProblems reports 10 most recent problematic signatures or unsigned commits.
func describeProblems(g *v1alpha1.SourceIntegrityGitPolicyGPG, signatureInfos []git.RevisionSignatureInfo) []string {
	reportedKeys := make(map[string]any)
	var problems []string
	for _, signatureInfo := range signatureInfos {
		// Do not report the same key twice unless:
		// - the revision is unsigned (unsigned commits can have different authors, so they are worth reporting)
		// - the revision is a tag (tags are signed separately from commits)
		if signatureInfo.SignatureKeyID != "" && git.IsCommitSHA(signatureInfo.Revision) {
			if _, exists := reportedKeys[signatureInfo.SignatureKeyID]; exists {
				continue
			}
			reportedKeys[signatureInfo.SignatureKeyID] = nil
		}

		problem := gpgProblemMessage(g, signatureInfo)
		if problem != "" {
			problems = append(problems, problem)

			// Report at most 10 problems
			if len(problems) >= 10 {
				break
			}
		}
	}
	return problems
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
	if isKeyInAllowedList(g.Keys, signatureInfo.SignatureKeyID) {
		return ""
	}
	return fmt.Sprintf("Failed verifying revision %s by '%s': "+msgUnallowedKey,
		signatureInfo.Revision, signatureInfo.AuthorIdentity, signatureInfo.SignatureKeyID,
	)
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

// CheckNameHelmProvenance is the name used for Helm provenance verification in SourceIntegrityCheckResult.
const CheckNameHelmProvenance = "HELM/PROVENANCE"

// VerifyHelm verifies Helm chart provenance when a matching policy requires it.
// chartContent is the raw .tgz bytes; provContent is the .prov file (may be nil if missing); chartFilename is e.g. "mychart-1.0.0.tgz".
// Returns nil when no policy matches or mode is none; otherwise returns a check result (HELM/PROVENANCE).
func VerifyHelm(si *v1alpha1.SourceIntegrity, repoURL string, chartContent []byte, provContent []byte, chartFilename string) (*v1alpha1.SourceIntegrityCheckResult, error) {
	policy, earlyResult := resolveHelmProvenancePolicy(si, repoURL)
	if earlyResult != nil {
		return earlyResult, nil
	}
	if policy == nil {
		return nil, nil
	}
	if !IsGPGEnabled() {
		return helmProvenanceResult(nil), nil
	}
	problems := verifyHelmProvenanceContent(policy, chartContent, provContent, chartFilename)
	return helmProvenanceResult(problems), nil
}

func helmProvenanceResult(problems []string) *v1alpha1.SourceIntegrityCheckResult {
	return &v1alpha1.SourceIntegrityCheckResult{Checks: []v1alpha1.SourceIntegrityCheckResultItem{{
		Name:     CheckNameHelmProvenance,
		Problems: problems,
	}}}
}

func resolveHelmProvenancePolicy(si *v1alpha1.SourceIntegrity, repoURL string) (*v1alpha1.SourceIntegrityHelmPolicy, *v1alpha1.SourceIntegrityCheckResult) {
	if si == nil || si.Helm == nil {
		return nil, nil
	}
	policies := findMatchingHelmPolicies(si.Helm, repoURL)
	if len(policies) == 0 {
		return nil, nil
	}
	if len(policies) > 1 {
		msg := fmt.Sprintf("multiple (%d) Helm source integrity policies found for repo URL: %s", len(policies), repoURL)
		log.Warn(msg)
		return nil, helmProvenanceResult([]string{msg})
	}
	policy := policies[0]
	if policy.Provenance == nil || policy.Provenance.Mode == v1alpha1.SourceIntegrityHelmPolicyProvenanceModeNone {
		return nil, nil
	}
	if policy.Provenance.Mode != v1alpha1.SourceIntegrityHelmPolicyProvenanceModeProvenance {
		return nil, helmProvenanceResult([]string{fmt.Sprintf("unknown Helm source integrity provenance mode %q", policy.Provenance.Mode)})
	}
	return policy, nil
}

// helmProvenanceVerifier verifies PGP-signed provenance and returns the signer key ID.
var helmProvenanceVerifier = VerifyCleartextSignedMessage

func verifyHelmProvenanceContent(policy *v1alpha1.SourceIntegrityHelmPolicy, chartContent []byte, provContent []byte, chartFilename string) []string {
	if len(provContent) == 0 {
		return []string{"provenance file (.prov) is required but missing"}
	}
	signerKeyID, err := helmProvenanceVerifier(provContent)
	if err != nil {
		return []string{"provenance signature verification failed: " + err.Error()}
	}
	if !isKeyInAllowedList(policy.Provenance.Keys, signerKeyID) {
		signerShort := signerKeyID
		if s, e := KeyID(signerKeyID); e == nil {
			signerShort = s
		}
		return []string{fmt.Sprintf(msgUnallowedKey, signerShort)}
	}
	signedBody, err := extractProvSignedBody(provContent)
	if err != nil {
		return []string{"failed to parse provenance signed body: " + err.Error()}
	}
	expectedSHA, err := parseProvFilesDigest(signedBody, chartFilename)
	if err != nil {
		return []string{err.Error()}
	}
	if err := verifyChartChecksum(chartContent, expectedSHA); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func getHelmPolicyURLs(si *v1alpha1.SourceIntegrityHelm) (urls []string) {
	for _, p := range si.Policies {
		for _, r := range p.Repos {
			urls = append(urls, r.URL)
		}
	}
	return urls
}

func findMatchingHelmPolicies(si *v1alpha1.SourceIntegrityHelm, repoURL string) (policies []*v1alpha1.SourceIntegrityHelmPolicy) {
	for _, p := range si.Policies {
		globs := make([]string, 0, len(p.Repos))
		for _, r := range p.Repos {
			globs = append(globs, r.URL)
		}
		if repoURLMatchesPolicyGlobs(globs, repoURL) {
			policies = append(policies, p)
		}
	}
	return policies
}

// lookupHelm returns non-nil if there is exactly one matching Helm policy that has verification (provenance).
func lookupHelm(si *v1alpha1.SourceIntegrity, repoURL string) *v1alpha1.SourceIntegrityHelmPolicy {
	policies := findMatchingHelmPolicies(si.Helm, repoURL)
	if len(policies) != 1 {
		return nil
	}
	p := policies[0]
	if p.Provenance == nil || p.Provenance.Mode == v1alpha1.SourceIntegrityHelmPolicyProvenanceModeNone {
		return nil
	}
	return p
}

const (
	pgpSignedMessageHeader = "-----BEGIN PGP SIGNED MESSAGE-----"
	pgpSignatureHeader     = "-----BEGIN PGP SIGNATURE-----"
)

// extractProvSignedBody extracts the signed body from a PGP cleartext-signed message (e.g. Helm .prov file).
func extractProvSignedBody(provContent []byte) ([]byte, error) {
	s := string(provContent)
	// Find the signature boundary
	idx := strings.Index(s, "\n"+pgpSignatureHeader)
	if idx < 0 {
		return nil, fmt.Errorf("provenance missing %s boundary", pgpSignatureHeader)
	}
	bodyWithHeader := strings.TrimSuffix(s[:idx], "\n")
	// Ignore "-----BEGIN PGP SIGNED MESSAGE-----" and "Hash: SHA256" and the blank line after.
	if !strings.HasPrefix(bodyWithHeader, pgpSignedMessageHeader) {
		return nil, fmt.Errorf("provenance missing %s", pgpSignedMessageHeader)
	}
	rest := bodyWithHeader[len(pgpSignedMessageHeader):]
	// Rest is "\nHash: SHA256\n\n" + body (or "\nHash: ...\n\n" + body)
	doubleNewline := strings.Index(rest, "\n\n")
	if doubleNewline < 0 {
		return nil, fmt.Errorf("provenance signed body has no blank line after Hash")
	}
	body := rest[doubleNewline+2:]
	return []byte(body), nil
}

// provFilesDigestRegex matches a line like "  helm-1.0.0.tgz: sha256:<64 hex chars>"
var provFilesDigestRegex = regexp.MustCompile(`(?m)^\s+([^:]+):\s+sha256:([0-9a-fA-F]{64})\s*$`)

// parseProvFilesDigest parses the provenance signed body (YAML-like "files:" section) and returns
// the expected SHA256 digest (64 hex chars) for the given chart filename.
func parseProvFilesDigest(signedBody []byte, chartFilename string) (expectedSHA256Hex string, err error) {
	matches := provFilesDigestRegex.FindAllSubmatch(signedBody, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			fn := string(m[1])
			if fn == chartFilename {
				return string(m[2]), nil
			}
		}
	}
	return "", fmt.Errorf("provenance files section has no digest for %q", chartFilename)
}

// verifyChartChecksum verifies that chartContent's SHA256 matches the expected hex digest.
func verifyChartChecksum(chartContent []byte, expectedSHA256Hex string) error {
	sum := sha256.Sum256(chartContent)
	got := hex.EncodeToString(sum[:])
	if got != expectedSHA256Hex {
		return fmt.Errorf("chart digest mismatch: got %s, provenance expects %s", got, expectedSHA256Hex)
	}
	return nil
}
