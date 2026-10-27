package e2e

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestCliCoreKubeConfigPersistence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "argocd-config")
	kubeConfigPath := filepath.Join(dir, "kubeconfig")
	reachedAPI := make(chan string, 32)
	// Stop at the Kubernetes API boundary, so this regression does not depend on
	// core-mode repo-server pods being present in the e2e installation.
	newKubeConfig := func(path string) *atomic.Int32 {
		calls := &atomic.Int32{}
		api := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			select {
			case reachedAPI <- path:
			default:
			}
			// Keep the first request open until the test stops the CLI process.
			// This prevents retries or later startup requests from entering the next probe.
			<-r.Context().Done()
		}))
		t.Cleanup(api.Close)
		require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
			Clusters:       map[string]*clientcmdapi.Cluster{"cluster": {Server: api.URL}},
			Contexts:       map[string]*clientcmdapi.Context{"context": {Cluster: "cluster", AuthInfo: "user", Namespace: "argocd"}},
			AuthInfos:      map[string]*clientcmdapi.AuthInfo{"user": {}},
			CurrentContext: "context",
		}, path))
		return calls
	}
	savedCalls := newKubeConfig(kubeConfigPath)
	overridePath := filepath.Join(dir, "override-kubeconfig")
	overrideCalls := newKubeConfig(overridePath)
	binary, err := filepath.Abs("../../dist/argocd")
	require.NoError(t, err)
	runCLI := func(workDir string, args ...string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	// An unusable default makes reaching the endpoint prove that the saved file was used.
	t.Setenv("KUBECONFIG", filepath.Join(dir, "missing-default"))
	output, err := runCLI(dir, "login", "--core", "--name", "custom-core", "--config", configPath, "--kubeconfig", "kubeconfig")
	require.NoError(t, err, "output: %s", output)
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	require.NoError(t, err)
	core, err := localCfg.ResolveContext("custom-core")
	require.NoError(t, err)
	assert.Equal(t, kubeConfigPath, core.KubeConfigPath)

	// Use another working directory and omit both --core and --kubeconfig.
	workDir := t.TempDir()
	runUntilAPI := func(wantPath string, flags ...string) {
		t.Helper()
		for len(reachedAPI) > 0 {
			<-reachedAPI
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		args := append([]string{"app", "list", "--config", configPath}, flags...)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = workDir
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		require.NoError(t, cmd.Start())
		var gotPath string
		select {
		case gotPath = <-reachedAPI:
		case <-ctx.Done():
		}
		// Stop once configuration selection is observed. The rest of core-mode
		// server startup is outside this configuration regression.
		cancel()
		_ = cmd.Wait()
		require.Equal(t, wantPath, gotPath, "CLI did not reach the selected API: %s", output.String())
	}
	runUntilAPI(kubeConfigPath)
	assert.Positive(t, savedCalls.Load())
	assert.Zero(t, overrideCalls.Load())
	savedBeforeOverride := savedCalls.Load()
	runUntilAPI(overridePath, "--kubeconfig", overridePath)
	assert.Positive(t, overrideCalls.Load())
	assert.Equal(t, savedBeforeOverride, savedCalls.Load())

	missingOverride := filepath.Join(dir, "missing-override")
	output, err = runCLI(workDir, "app", "list", "--config", configPath, "--kubeconfig", missingOverride)
	require.Error(t, err)
	assert.Contains(t, output, "missing-override")

	runUntilAPI(kubeConfigPath)
	assert.Greater(t, savedCalls.Load(), savedBeforeOverride, "a one-off override must not replace the saved path")
}
