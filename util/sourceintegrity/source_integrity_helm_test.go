package sourceintegrity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	helmRepoURL     = "https://charts.example.com/stable"
	provChartName   = "helm-provenance"
	provChartFile   = "helm-provenance-1.0.0.tgz"
	provChartDigest = "sha256:a7299904d85a05ce9e5ea07ec6118721a53cbca4b4bf784b0ed1dcbe3f7643bf" // of provChartFile
	provSignerKeyID = "D56C4FCA57A46444"                                                        // test/fixture/gpg/D56C4FCA57A46444
	provSubkeyID    = "1111111111111111"
	unrelatedKeyID  = "0000000000000000"
)

func helmSI(policies ...*v1alpha1.SourceIntegrityHelmPolicy) *v1alpha1.SourceIntegrity {
	return &v1alpha1.SourceIntegrity{Helm: &v1alpha1.SourceIntegrityHelm{Policies: policies}}
}

func provenancePolicy(repoGlob string, keys ...string) *v1alpha1.SourceIntegrityHelmPolicy {
	return &v1alpha1.SourceIntegrityHelmPolicy{
		Repos:      []v1alpha1.SourceIntegrityHelmPolicyRepo{{URL: repoGlob}},
		Provenance: &v1alpha1.SourceIntegrityHelmPolicyProvenance{Keys: keys},
	}
}

func readProv(t *testing.T) []byte {
	t.Helper()
	prov, err := os.ReadFile("testdata/" + provChartFile + ".prov")
	require.NoError(t, err)
	return prov
}

// provFile writes prov to a temporary .prov file and returns its path.
func provFile(t *testing.T, prov []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), provChartFile+".prov")
	require.NoError(t, os.WriteFile(path, prov, 0o600))
	return path
}

// stubVerifyClearsigned makes every signature verify as signed by signingKeyID (subkey of primaryKeyID).
func stubVerifyClearsigned(t *testing.T, signingKeyID, primaryKeyID string) {
	t.Helper()
	orig := verifyClearsigned
	t.Cleanup(func() { verifyClearsigned = orig })
	verifyClearsigned = func(context.Context, []byte) (string, string, error) { return signingKeyID, primaryKeyID, nil }
}

func TestVerifyHelm(t *testing.T) {
	prov := readProv(t)
	fetchChart := func() (string, string, error) { return provChartDigest, provFile(t, prov), nil }

	tests := []struct {
		name        string
		si          *v1alpha1.SourceIntegrity
		fetch       func() (string, string, error)
		chart       string // requested; provChartName unless set
		version     string // requested; "1.0.0" unless set, "-" for none
		signer      string
		primary     string
		wantNil     bool
		wantErr     string
		wantProblem string
	}{{
		name:    "no source integrity",
		si:      nil,
		wantNil: true,
	}, {
		name:    "git only",
		si:      &v1alpha1.SourceIntegrity{Git: &v1alpha1.SourceIntegrityGit{}},
		wantNil: true,
	}, {
		name:    "no policy matches the repo",
		si:      helmSI(provenancePolicy("https://other.example.com/*", provSignerKeyID)),
		wantNil: true,
	}, {
		name:    "negative glob excludes the repo",
		si:      helmSI(&v1alpha1.SourceIntegrityHelmPolicy{Repos: []v1alpha1.SourceIntegrityHelmPolicyRepo{{URL: "*"}, {URL: "!" + helmRepoURL}}, Provenance: &v1alpha1.SourceIntegrityHelmPolicyProvenance{}}),
		wantNil: true,
	}, {
		name:    "matching policy without provenance",
		si:      helmSI(&v1alpha1.SourceIntegrityHelmPolicy{Repos: []v1alpha1.SourceIntegrityHelmPolicyRepo{{URL: "*"}}}),
		wantNil: true,
	}, {
		name:        "multiple matching policies fail",
		si:          helmSI(provenancePolicy("*", provSignerKeyID), provenancePolicy(helmRepoURL, provSignerKeyID)),
		wantProblem: "multiple (2) Helm source integrity policies found for repo URL: " + helmRepoURL,
	}, {
		name:   "valid provenance signed by allowed key",
		si:     helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:  fetchChart,
		signer: provSignerKeyID,
	}, {
		name:    "signed by subkey of allowed primary key",
		si:      helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:   fetchChart,
		signer:  provSubkeyID,
		primary: provSignerKeyID,
	}, {
		name:        "signed by key not in policy",
		si:          helmSI(provenancePolicy("*", unrelatedKeyID)),
		fetch:       fetchChart,
		signer:      provSignerKeyID,
		wantProblem: "signed with unallowed key (key_id=" + provSignerKeyID + ")",
	}, {
		name:        "empty key list rejects every signer",
		si:          helmSI(provenancePolicy("*")),
		fetch:       fetchChart,
		signer:      provSignerKeyID,
		wantProblem: "signed with unallowed key (key_id=" + provSignerKeyID + ")",
	}, {
		name:        "chart does not match signed digest",
		si:          helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:       func() (string, string, error) { return "sha256:0000", provFile(t, prov), nil },
		signer:      provSignerKeyID,
		wantProblem: "chart digest mismatch: provenance does not sign sha256:0000",
	}, {
		name:        "signed metadata names another chart",
		si:          helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:       fetchChart,
		chart:       "other-chart",
		signer:      provSignerKeyID,
		wantProblem: `provenance signs chart "helm-provenance", not "other-chart"`,
	}, {
		name:        "signed metadata names another version",
		si:          helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:       fetchChart,
		version:     "2.0.0",
		signer:      provSignerKeyID,
		wantProblem: `provenance signs version "1.0.0" of the chart, not "2.0.0"`,
	}, {
		name:    "version not checked when unknown",
		si:      helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:   fetchChart,
		version: "-",
		signer:  provSignerKeyID,
	}, {
		name:        "trailing data after the signed block",
		si:          helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:       func() (string, string, error) { return provChartDigest, provFile(t, append(prov, "extra"...)), nil },
		signer:      provSignerKeyID,
		wantProblem: "provenance is not a single PGP cleartext-signed message",
	}, {
		name:        "empty provenance",
		si:          helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:       func() (string, string, error) { return provChartDigest, provFile(t, nil), nil },
		wantProblem: "provenance file (.prov) is required but missing",
	}, {
		name: "provenance not downloaded",
		si:   helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch: func() (string, string, error) {
			return provChartDigest, filepath.Join(t.TempDir(), "missing.prov"), nil
		},
		wantErr: "could not access chart for provenance verification: chart has no provenance file (.prov)",
	}, {
		name:    "oversized provenance",
		si:      helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:   func() (string, string, error) { return provChartDigest, provFile(t, make([]byte, 1<<20+1)), nil },
		wantErr: "could not access chart for provenance verification: provenance file (.prov) is more than the 1048576 bytes allowed",
	}, {
		name:    "fetch error is returned, not cached as a verdict",
		si:      helmSI(provenancePolicy("*", provSignerKeyID)),
		fetch:   func() (string, string, error) { return "", "", errors.New("502 Bad Gateway") },
		wantErr: "could not access chart for provenance verification: 502 Bad Gateway",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubVerifyClearsigned(t, tt.signer, tt.primary)
			fetch := tt.fetch
			if fetch == nil {
				fetch = func() (string, string, error) {
					t.Fatal("fetch must not be called")
					return "", "", nil
				}
			}

			version := tt.version
			switch version {
			case "":
				version = "1.0.0"
			case "-":
				version = ""
			}
			chart := tt.chart
			if chart == "" {
				chart = provChartName
			}
			result, err := VerifyHelm(t.Context(), tt.si, helmRepoURL, chart, version, fetch)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, v1alpha1.SourceIntegrityMethodNameHelmProvenance, result.Checks[0].Name)
			if tt.wantProblem == "" {
				assert.Empty(t, result.Checks[0].Problems)
			} else {
				require.Len(t, result.Checks[0].Problems, 1)
				assert.Contains(t, result.Checks[0].Problems[0], tt.wantProblem)
			}
		})
	}
}

func TestVerifyHelmReturnsGPGRunFailure(t *testing.T) {
	orig := verifyClearsigned
	t.Cleanup(func() { verifyClearsigned = orig })
	verifyClearsigned = func(context.Context, []byte) (string, string, error) {
		return "", "", fmt.Errorf("%w: %w", errNoSignatureStatus, context.DeadlineExceeded)
	}
	prov := readProv(t)
	result, err := VerifyHelm(t.Context(), helmSI(provenancePolicy("*", provSignerKeyID)), helmRepoURL, provChartName, "1.0.0", func() (string, string, error) {
		return provChartDigest, provFile(t, prov), nil
	})
	require.ErrorIs(t, err, context.DeadlineExceeded, "not cached as a failed check")
	assert.Nil(t, result)
}

func TestVerifyHelmSkipsWhenGPGDisabled(t *testing.T) {
	t.Setenv("ARGOCD_GPG_ENABLED", "false")
	result, err := VerifyHelm(t.Context(), helmSI(provenancePolicy("*", provSignerKeyID)), helmRepoURL, provChartName, "1.0.0", func() (string, string, error) {
		t.Fatal("fetch must not be called")
		return "", "", nil
	})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestHasCriteriaHelm(t *testing.T) {
	helmSource := v1alpha1.ApplicationSource{RepoURL: helmRepoURL, Chart: "helm-provenance"}
	gitSource := v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo.git", Path: "."}

	assert.True(t, HasCriteria(helmSI(provenancePolicy("*")), helmSource))
	assert.False(t, HasCriteria(helmSI(provenancePolicy("https://other.example.com/*")), helmSource))
	assert.False(t, HasCriteria(helmSI(provenancePolicy("*")), gitSource), "Helm policies do not apply to git sources")
	assert.True(t, HasCriteria(helmSI(provenancePolicy("*")), gitSource, helmSource))

	assert.True(t, HasCriteria(helmSI(&v1alpha1.SourceIntegrityHelmPolicy{Repos: []v1alpha1.SourceIntegrityHelmPolicyRepo{{URL: "*"}}}, &v1alpha1.SourceIntegrityHelmPolicy{Repos: []v1alpha1.SourceIntegrityHelmPolicyRepo{{URL: "*"}}}), helmSource), "VerifyHelm fails when several policies match")

	t.Setenv("ARGOCD_GPG_ENABLED", "false")
	assert.False(t, HasCriteria(helmSI(provenancePolicy("*")), helmSource), "VerifyHelm skips when GnuPG is disabled")
	assert.True(t, HasCriteria(helmSI(provenancePolicy("*"), provenancePolicy("*")), helmSource), "VerifyHelm fails several matching policies even with GnuPG disabled")
}

func TestParseGPGVerifyStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		wantSigning string
		wantPrimary string
		wantErr     string
	}{{
		name: "good signature by subkey",
		status: `[GNUPG:] NEWSIG
[GNUPG:] GOODSIG 1111111111111111 Signer <signer@example.com>
[GNUPG:] VALIDSIG ABCDABCDABCDABCDABCDABCD1111111111111111 2026-01-01 1767225600 0 4 0 1 10 01 EA459B49595CBE3FD1FBA303D56C4FCA57A46444
[GNUPG:] TRUST_ULTIMATE 0 pgp
`,
		wantSigning: "1111111111111111",
		wantPrimary: provSignerKeyID,
	}, {
		name:        "good signature reported by fingerprint",
		status:      "[GNUPG:] GOODSIG EA459B49595CBE3FD1FBA303D56C4FCA57A46444 Signer\n[GNUPG:] TRUST_FULLY 0 pgp\n",
		wantSigning: provSignerKeyID,
	}, {
		name:    "good signature by untrusted key",
		status:  "[GNUPG:] GOODSIG D56C4FCA57A46444 Signer\n[GNUPG:] TRUST_UNDEFINED 0 pgp\n",
		wantErr: "signed with untrusted key (key_id=D56C4FCA57A46444)",
	}, {
		name:    "bad signature",
		status:  "[GNUPG:] BADSIG D56C4FCA57A46444 Signer\n",
		wantErr: "bad signature (key_id=D56C4FCA57A46444)",
	}, {
		name:    "key not in keyring",
		status:  "[GNUPG:] ERRSIG D56C4FCA57A46444 1 10 00 1763632400 9 EA459B49595CBE3FD1FBA303D56C4FCA57A46444\n[GNUPG:] NO_PUBKEY D56C4FCA57A46444\n",
		wantErr: "signed with key not in keyring (key_id=D56C4FCA57A46444)",
	}, {
		name:    "expired key",
		status:  "[GNUPG:] EXPKEYSIG D56C4FCA57A46444 Signer\n",
		wantErr: "signed with expired key (key_id=D56C4FCA57A46444)",
	}, {
		name:    "two signatures",
		status:  "[GNUPG:] GOODSIG D56C4FCA57A46444 A\n[GNUPG:] GOODSIG 1111111111111111 B\n",
		wantErr: "provenance must carry exactly one signature",
	}, {
		name:    "not a signed message",
		status:  "[GNUPG:] NODATA 1\n",
		wantErr: "not a PGP signed message",
	}, {
		name:    "no status",
		status:  "",
		wantErr: "gpg reported no signature status",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signing, primary, err := parseGPGVerifyStatus(tt.status)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSigning, signing)
			assert.Equal(t, tt.wantPrimary, primary)
		})
	}
}

func TestParseProvenance(t *testing.T) {
	// helm package --sign separates the chart metadata and files documents with "...".
	body := []byte("apiVersion: v2\nname: helm-provenance\nversion: 1.0.0\n\n...\nfiles:\n  helm-provenance-1.0.0.tgz: sha256:abc123\n")
	name, version, files, err := parseProvenance(body)
	require.NoError(t, err)
	assert.Equal(t, provChartName, name)
	assert.Equal(t, "1.0.0", version)
	assert.Equal(t, map[string]string{provChartFile: "sha256:abc123"}, files)

	_, _, _, err = parseProvenance([]byte("apiVersion: v2\n"))
	require.ErrorContains(t, err, "no files section")
}

func TestGPGVerifyClearsigned(t *testing.T) {
	initTempDir(t)
	require.NoError(t, InitializeGnuPG())
	prov := readProv(t)

	_, _, err := gpgVerifyClearsigned(t.Context(), prov)
	require.ErrorContains(t, err, "signed with key not in keyring (key_id="+provSignerKeyID+")")

	_, _, err = gpgVerifyClearsigned(t.Context(), []byte("not a provenance file\n"))
	require.EqualError(t, err, "not a PGP signed message")

	_, err = ImportPGPKeys("../../test/fixture/gpg/" + provSignerKeyID)
	require.NoError(t, err)
	_, _, err = gpgVerifyClearsigned(t.Context(), prov)
	require.ErrorContains(t, err, "signed with untrusted key (key_id="+provSignerKeyID+")")

	// The keyring sync gives every configured key ultimate trust.
	require.NoError(t, SetPGPTrustLevelById([]string{"EA459B49595CBE3FD1FBA303D56C4FCA57A46444"}, TrustUltimate))
	signing, primary, err := gpgVerifyClearsigned(t.Context(), prov)
	require.NoError(t, err)
	assert.Equal(t, provSignerKeyID, signing)
	assert.Equal(t, provSignerKeyID, primary)
}
