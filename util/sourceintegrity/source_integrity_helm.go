package sourceintegrity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

// errNoSignatureStatus means gpg did not get as far as checking a signature, for example because
// it could not run. That is not a verdict on the chart, so it is returned as an error.
var errNoSignatureStatus = errors.New("gpg reported no signature status")

// VerifyHelm verifies a Helm chart against the provenance policy matching repoURL. It returns nil
// when no policy applies. fetch is called only when one does; it returns the digest of the chart
// archive ("sha256:<hex>") and the path of its .prov file. The signed chart metadata must name
// chartName and, unless it is empty, chartVersion, so a signed but different chart (such as an older
// version) cannot be served in place of the requested one.
//
// Failing to get the chart or its .prov is returned as an error rather than a failed check, so the
// repo-server does not cache what may be a transient download failure as a verdict.
func VerifyHelm(ctx context.Context, si *v1alpha1.SourceIntegrity, repoURL, chartName, chartVersion string, fetch func() (chartDigest, provPath string, err error)) (*v1alpha1.SourceIntegrityCheckResult, error) {
	if si == nil || si.Helm == nil {
		return nil, nil
	}
	policies := findMatchingHelmPolicies(si.Helm, repoURL)
	if len(policies) == 0 {
		return nil, nil
	}
	if len(policies) > 1 {
		// Fail rather than skip, so a policy misconfiguration does not disable verification.
		msg := fmt.Sprintf("multiple (%d) Helm source integrity policies found for repo URL: %s", len(policies), repoURL)
		log.Warn(msg)
		return helmProvenanceResult(msg), nil
	}
	provenance := policies[0].Provenance
	if provenance == nil {
		return nil, nil
	}
	if !IsGPGEnabled() {
		log.Warn("SourceIntegrity criteria for helm provenance declared, but it is turned off by ARGOCD_GPG_ENABLED")
		return nil, nil
	}

	chartDigest, provPath, err := fetch()
	if err != nil {
		return nil, fmt.Errorf("could not access chart for provenance verification: %w", err)
	}
	prov, err := readProvenance(provPath)
	if err != nil {
		return nil, fmt.Errorf("could not access chart for provenance verification: %w", err)
	}
	problem, err := helmProvenanceProblem(ctx, provenance.Keys, chartName, chartVersion, chartDigest, prov)
	if err != nil {
		return nil, fmt.Errorf("helm provenance verification: %w", err)
	}
	return helmProvenanceResult(problem), nil
}

// readProvenance reads a downloaded .prov file, refusing implausibly large ones (real ones are a few KiB).
func readProvenance(path string) ([]byte, error) {
	// The .prov sits next to extracted archive content, so it is opened through an os.Root.
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(filepath.Base(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("chart has no provenance file (.prov)")
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	prov, err := io.ReadAll(io.LimitReader(f, helm.MaxProvenanceSize+1))
	if err != nil {
		return nil, err
	}
	if len(prov) > helm.MaxProvenanceSize {
		return nil, fmt.Errorf("provenance file (.prov) is more than the %d bytes allowed", helm.MaxProvenanceSize)
	}
	return prov, nil
}

func hasHelmCriteria(si *v1alpha1.SourceIntegrity, repoURL string) bool {
	if si.Helm == nil {
		return false
	}
	// In the same order as VerifyHelm: several matching policies fail even with GnuPG disabled.
	policies := findMatchingHelmPolicies(si.Helm, repoURL)
	if len(policies) > 1 {
		return true
	}
	return len(policies) == 1 && policies[0].Provenance != nil && IsGPGEnabled()
}

func findMatchingHelmPolicies(si *v1alpha1.SourceIntegrityHelm, repoURL string) (policies []*v1alpha1.SourceIntegrityHelmPolicy) {
	for _, p := range si.Policies {
		include := false
		for _, r := range p.Repos {
			m := repoMatches(r.URL, repoURL)
			if m == -1 {
				include = false
				break
			} else if m == 1 {
				include = true
			}
		}
		if include {
			policies = append(policies, p)
		}
	}
	return policies
}

// helmProvenanceResult returns a HELM/PROVENANCE check that passes when problem is empty.
func helmProvenanceResult(problem string) *v1alpha1.SourceIntegrityCheckResult {
	item := v1alpha1.SourceIntegrityCheckResultItem{Name: v1alpha1.SourceIntegrityMethodNameHelmProvenance}
	if problem != "" {
		item.Problems = []string{problem}
	}
	return &v1alpha1.SourceIntegrityCheckResult{Checks: []v1alpha1.SourceIntegrityCheckResultItem{item}}
}

// helmProvenanceProblem performs the checks of `helm verify`: the .prov signature is good and made
// by an allowed key, and it signs chartDigest. It returns "" on success.
func helmProvenanceProblem(ctx context.Context, keys []string, chartName, chartVersion, chartDigest string, prov []byte) (string, error) {
	if len(prov) == 0 {
		return "provenance file (.prov) is required but missing", nil
	}
	signingKeyID, primaryKeyID, err := verifyClearsigned(ctx, prov)
	if errors.Is(err, errNoSignatureStatus) {
		return "", err
	} else if err != nil {
		return "provenance signature verification failed: " + err.Error(), nil
	}
	if !keyAllowed(keys, signingKeyID) && !keyAllowed(keys, primaryKeyID) {
		return fmt.Sprintf("signed with unallowed key (key_id=%s)", signingKeyID), nil
	}
	block, rest := clearsign.Decode(prov)
	if block == nil || len(bytes.TrimSpace(rest)) > 0 {
		// gpg verified the whole file; read the digest only from a file that is exactly one signed block.
		return "provenance is not a single PGP cleartext-signed message", nil
	}
	signedName, signedVersion, files, err := parseProvenance(block.Plaintext)
	if err != nil {
		return err.Error(), nil
	}
	// Match the digest rather than the archive name, which depends on where the chart was fetched
	// from, and check the chart it belongs to against the signed metadata instead.
	signed := false
	for d := range maps.Values(files) {
		signed = signed || d == chartDigest
	}
	if !signed {
		return "chart digest mismatch: provenance does not sign " + chartDigest, nil
	}
	if signedName != chartName {
		return fmt.Sprintf("provenance signs chart %q, not %q", signedName, chartName), nil
	}
	if chartVersion != "" && signedVersion != chartVersion {
		return fmt.Sprintf("provenance signs version %q of the chart, not %q", signedVersion, chartVersion), nil
	}
	return "", nil
}

// parseProvenance returns the chart name and version and the file name to digest ("sha256:<hex>")
// map that a provenance body signs. Helm writes the chart metadata and the files section as two
// YAML documents split by "...".
func parseProvenance(body []byte) (name, version string, files map[string]string, err error) {
	parts := bytes.Split(body, []byte("\n...\n"))
	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := yaml.Unmarshal(parts[0], &metadata); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse provenance chart metadata: %w", err)
	}
	var doc struct {
		Files map[string]string `json:"files"`
	}
	if err := yaml.Unmarshal(parts[len(parts)-1], &doc); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse provenance files section: %w", err)
	}
	if len(doc.Files) == 0 {
		return "", "", nil, errors.New("provenance has no files section")
	}
	return metadata.Name, metadata.Version, doc.Files, nil
}

// verifyClearsigned is a variable so tests can run without a GnuPG keyring.
var verifyClearsigned = gpgVerifyClearsigned

// gpgVerifyClearsigned checks a cleartext-signed message against the Argo CD keyring and returns
// the signing key ID and its primary key ID (they differ when a subkey signed).
func gpgVerifyClearsigned(ctx context.Context, msg []byte) (signingKeyID, primaryKeyID string, err error) {
	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--status-fd", "1", "--verify")
	cmd.Env = getGPGEnviron()
	cmd.Stdin = bytes.NewReader(msg)
	// gpg exits non-zero for bad signatures; the status lines say why, so parse them first.
	out, runErr := executil.RunWithExecRunOpts(cmd, executil.ExecRunOpts{SkipErrorLogging: true})
	signingKeyID, primaryKeyID, err = parseGPGVerifyStatus(out)
	switch {
	case errors.Is(err, errNoSignatureStatus) && runErr != nil:
		err = fmt.Errorf("%w: %w", err, runErr)
	case err == nil && runErr != nil:
		err = fmt.Errorf("gpg reported a good signature but failed: %w", runErr)
	}
	return signingKeyID, primaryKeyID, err
}

// parseGPGVerifyStatus interprets `gpg --status-fd` output for a single signature.
// See https://github.com/gpg/gnupg/blob/master/doc/DETAILS#general-status-codes
func parseGPGVerifyStatus(status string) (signingKeyID, primaryKeyID string, err error) {
	failures := map[string]string{
		"BADSIG":    "bad signature",
		"EXPSIG":    "expired signature",
		"EXPKEYSIG": "signed with expired key",
		"REVKEYSIG": "signed with revoked key",
		"ERRSIG":    "signed with key not in keyring",
	}
	var code, keyID string
	var trusted, noData bool
	for line := range strings.SplitSeq(status, "\n") {
		fields := strings.Fields(strings.TrimPrefix(line, "[GNUPG:] "))
		if len(fields) < 2 {
			continue
		}
		switch _, failure := failures[fields[0]]; {
		case fields[0] == "GOODSIG" || failure:
			if code != "" {
				return "", "", errors.New("provenance must carry exactly one signature")
			}
			code = fields[0]
			// GOODSIG and friends carry a long key ID or a fingerprint.
			keyID = fields[1]
			if id, err := KeyID(keyID); err == nil {
				keyID = id
			}
		case fields[0] == "NODATA":
			noData = true
		case fields[0] == "TRUST_FULLY" || fields[0] == "TRUST_ULTIMATE":
			// As for Git commits, a good signature by a key without full trust does not count.
			trusted = true
		case fields[0] == "VALIDSIG" && len(fields) >= 11:
			// The last field is the primary key fingerprint; KeyID takes its last 16 hex digits.
			primaryKeyID, _ = KeyID(fields[10])
		}
	}
	switch {
	case code == "" && noData:
		// gpg read the input and found no signature in it: a verdict on the file, not a gpg failure.
		return "", "", errors.New("not a PGP signed message")
	case code == "":
		return "", "", fmt.Errorf("%w: %q", errNoSignatureStatus, status)
	case code != "GOODSIG":
		return "", "", fmt.Errorf("%s (key_id=%s)", failures[code], keyID)
	case !trusted:
		return "", "", fmt.Errorf("signed with untrusted key (key_id=%s)", keyID)
	}
	return keyID, primaryKeyID, nil
}
