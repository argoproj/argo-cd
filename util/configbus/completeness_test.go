package configbus

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inventoryFile struct {
	CMKeys  []string `json:"cmKeys"`
	EnvVars []struct {
		Name      string `json:"name"`
		FlagBound bool   `json:"flagBound"`
	} `json:"envVars"`
	CmdParams []struct {
		CMKey  string `json:"cmKey"`
		EnvVar string `json:"envVar"`
	} `json:"cmdParams"`
	Flags []struct {
		Name      string `json:"name"`
		Component string `json:"component"`
		PureFlag  bool   `json:"pureFlag"`
	} `json:"flags"`
}

// standaloneEnvVars returns env var names that have at least one non-flag-bound
// read. Flag-bound-only vars (and cmd-param transport env vars) are documented
// via their argocd-cmd-params-cm key, so the provider does not own them.
func standaloneEnvVars(inv inventoryFile) map[string]bool {
	out := map[string]bool{}
	for _, e := range inv.EnvVars {
		if !e.FlagBound {
			out[e.Name] = true
		}
	}
	return out
}

func TestRegistryCompleteness(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	invData, err := os.ReadFile(filepath.Join(dir, "testdata", "inventory.json"))
	require.NoError(t, err)
	var inv inventoryFile
	require.NoError(t, json.Unmarshal(invData, &inv))

	allow := loadAllowlist(t, filepath.Join(dir, "testdata", "unregistered_allowlist.txt"))
	flagAllow := loadAllowlist(t, filepath.Join(dir, "testdata", "unregistered_flag_allowlist.txt"))

	cmKeys := map[string]bool{}
	for _, k := range inv.CMKeys {
		cmKeys[k] = true
	}
	for _, p := range inv.CmdParams {
		cmKeys[p.CMKey] = true
	}
	envKeys := standaloneEnvVars(inv)

	// First-slice keys must be registered and must not be allowlisted.
	assert.True(t, DescriptorCoversCMKey(CMKeyTimeoutReconciliation), "timeout.reconciliation must be registered")
	assert.True(t, DescriptorCoversEnv(EnvReconciliationTimeout), "ARGOCD_RECONCILIATION_TIMEOUT must be registered")
	assert.True(t, DescriptorCoversCMKey(CMKeyResourceCustomizations), "resource.customizations must be registered")
	assert.False(t, allow["cm:"+CMKeyTimeoutReconciliation], "first-slice key must not be allowlisted")
	assert.False(t, allow["env:"+EnvReconciliationTimeout], "first-slice env must not be allowlisted")
	assert.False(t, allow["cm:"+CMKeyResourceCustomizations], "first-slice key must not be allowlisted")

	var missing []string
	for k := range cmKeys {
		if DescriptorCoversCMKey(k) {
			continue
		}
		if allow["cm:"+k] {
			continue
		}
		// Dynamic prefix families: any key under a registered prefix is covered.
		covered := false
		for _, d := range AllDescriptors() {
			if d.CoversCMKey(k) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		missing = append(missing, "cm:"+k)
	}
	for e := range envKeys {
		if DescriptorCoversEnv(e) || allow["env:"+e] {
			continue
		}
		missing = append(missing, "env:"+e)
	}
	for _, f := range inv.Flags {
		if !f.PureFlag {
			continue
		}
		entry := "flag:" + f.Component + ":" + f.Name
		if DescriptorCoversFlag(f.Name) || flagAllow[entry] {
			continue
		}
		missing = append(missing, entry)
	}

	if len(missing) > 0 {
		t.Fatalf("unregistered config sources (add registry entries or allowlist):\n  %s", strings.Join(missing, "\n  "))
	}

	// Stale allowlist: registered keys should not remain allowlisted.
	var stale []string
	for entry := range allow {
		switch {
		case strings.HasPrefix(entry, "cm:"):
			if DescriptorCoversCMKey(strings.TrimPrefix(entry, "cm:")) {
				stale = append(stale, entry)
			}
		case strings.HasPrefix(entry, "env:"):
			if DescriptorCoversEnv(strings.TrimPrefix(entry, "env:")) {
				stale = append(stale, entry)
			}
		}
	}
	for entry := range flagAllow {
		if !strings.HasPrefix(entry, "flag:") {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) == 3 && DescriptorCoversFlag(parts[2]) {
			stale = append(stale, entry)
		}
	}
	assert.Empty(t, stale, "remove registered keys from allowlists")
}

func loadAllowlist(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	require.NoError(t, sc.Err())
	return out
}

func TestRegisteredDescriptorsAreHonest(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)
	invData, err := os.ReadFile(filepath.Join(dir, "testdata", "inventory.json"))
	require.NoError(t, err)
	var inv inventoryFile
	require.NoError(t, json.Unmarshal(invData, &inv))

	cmKeys := map[string]bool{}
	for _, k := range inv.CMKeys {
		cmKeys[k] = true
	}
	for _, p := range inv.CmdParams {
		cmKeys[p.CMKey] = true
	}
	envKeys := map[string]bool{}
	for _, e := range inv.EnvVars {
		envKeys[e.Name] = true
	}
	for _, p := range inv.CmdParams {
		if p.EnvVar != "" {
			envKeys[p.EnvVar] = true
		}
	}

	for _, d := range AllDescriptors() {
		if exact := d.CMKeyExact(); exact != "" {
			assert.Truef(t, cmKeys[exact], "registered CMKeyExact %q not found in inventory", exact)
		}
		if env := d.EnvVar(); env != "" {
			assert.Truef(t, envKeys[env], "registered EnvVar %q not found in inventory", env)
		}
		if prefix := d.CMKeyPrefix(); prefix != "" {
			found := false
			for k := range cmKeys {
				if k == strings.TrimSuffix(prefix, ".") || strings.HasPrefix(k, prefix) {
					found = true
					break
				}
			}
			assert.Truef(t, found, "registered CMKeyPrefix %q has no inventory keys", prefix)
		}
	}
}
