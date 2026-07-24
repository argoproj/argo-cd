package sourceintegrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/gpg"
)

func hasHelmProvenanceCriteriaForSource(si *v1alpha1.SourceIntegrity, source v1alpha1.ApplicationSource) bool {
	if !source.IsHelm() || source.IsHelmOci() || si == nil || si.Helm == nil {
		return false
	}
	policies := findMatchingHelmPolicies(si.Helm, source.RepoURL)
	for _, p := range policies {
		if p.Provenance != nil {
			return true
		}
	}
	return false
}

// CheckNameHelmProvenance is the name used for Helm provenance verification in SourceIntegrityCheckResult.
const CheckNameHelmProvenance = "HELM/PROVENANCE"

// VerifyHelm verifies Helm chart provenance when a matching policy requires it.
// chartContent is the raw .tgz bytes; provContent is the .prov file (may be nil if missing); chartFilename is e.g. "mychart-1.0.0.tgz".
// Returns nil when no policy matches or provenance is not configured; otherwise returns a check result (HELM/PROVENANCE).
func VerifyHelm(ctx context.Context, si *v1alpha1.SourceIntegrity, repoURL string, chartContent []byte, provContent []byte, chartFilename string) (*v1alpha1.SourceIntegrityCheckResult, error) {
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
	problems := verifyHelmProvenanceContent(ctx, policy, chartContent, provContent, chartFilename)
	return helmProvenanceResult(problems), nil
}

// HelmProvenanceFetchFailed returns a HELM/PROVENANCE check result when the chart
// or provenance data cannot be retrieved for verification.
// Policy resolution follows resolveHelmProvenancePolicy, including handling:
//   - multiple matching policies,
//   - or no matching provenance policy.
//
// If no provenance policy applies, the function returns nil.
func HelmProvenanceFetchFailed(si *v1alpha1.SourceIntegrity, repoURL string, cause error) *v1alpha1.SourceIntegrityCheckResult {
	policy, earlyResult := resolveHelmProvenancePolicy(si, repoURL)
	if earlyResult != nil {
		return earlyResult
	}
	if policy == nil {
		return nil
	}
	msg := fmt.Sprintf("could not access chart for provenance verification: %v", cause)
	return helmProvenanceResult([]string{msg})
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
	if policy.Provenance == nil {
		return nil, nil
	}
	return policy, nil
}

// helmProvenanceVerifier verifies PGP-signed provenance and returns the signer key ID.
var helmProvenanceVerifier = gpg.VerifyCleartextSignedMessage

func verifyHelmProvenanceContent(ctx context.Context, policy *v1alpha1.SourceIntegrityHelmPolicy, chartContent []byte, provContent []byte, chartFilename string) []string {
	if len(provContent) == 0 {
		return []string{"provenance file (.prov) is required but missing"}
	}
	signerKeyID, err := helmProvenanceVerifier(ctx, provContent)
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

// extractProvSignedBody extracts the signed body from a PGP cleartext-signed message (e.g. Helm .prov file).
// Uses the openpgp/clearsign library to properly parse the cleartext-signed message format.
func extractProvSignedBody(provContent []byte) ([]byte, error) {
	block, _ := clearsign.Decode(provContent)
	if block == nil {
		return nil, errors.New("provenance is not a valid PGP cleartext-signed message")
	}
	// block.Plaintext contains the signed body (the content between headers and signature)
	return block.Plaintext, nil
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
	if !strings.EqualFold(got, expectedSHA256Hex) {
		return fmt.Errorf("chart digest mismatch: got %s, provenance expects %s", got, expectedSHA256Hex)
	}
	return nil
}
