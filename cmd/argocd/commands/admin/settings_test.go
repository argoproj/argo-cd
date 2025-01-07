package admin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/common"
	utils "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func captureStdout(callback func()) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	callback()
	utils.Close(w)

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func newSettingsManager(data map[string]string) *settings.SettingsManager {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      common.ArgoCDConfigMapName,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      common.ArgoCDSecretName,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	return settings.NewSettingsManager(ctx, clientset, "default")
}

type fakeCmdContext struct {
	mgr *settings.SettingsManager
	// nolint:unused,structcheck
	out bytes.Buffer
}

func newCmdContext(data map[string]string) *fakeCmdContext {
	return &fakeCmdContext{mgr: newSettingsManager(data)}
}

func (ctx *fakeCmdContext) createSettingsManager(context.Context) (*settings.SettingsManager, error) {
	return ctx.mgr, nil
}

type validatorTestCase struct {
	validator       string
	data            map[string]string
	containsSummary string
	containsError   string
}

func TestCreateSettingsManager(t *testing.T) {
	ctx := context.Background()

	f, closer, err := tempFile(`apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  url: https://myargocd.com`)
	require.NoError(t, err)
	defer utils.Close(closer)

	opts := settingsOpts{argocdCMPath: f}
	settingsManager, err := opts.createSettingsManager(ctx)

	require.NoError(t, err)

	argoCDSettings, err := settingsManager.GetSettings()
	require.NoError(t, err)

	assert.Equal(t, "https://myargocd.com", argoCDSettings.URL)
}

func TestValidator(t *testing.T) {
	testCases := map[string]validatorTestCase{
		"General_SSOIsNotConfigured": {
			validator: "general", containsSummary: "SSO is not configured",
		},
		"General_DexInvalidConfig": {
			validator:     "general",
			data:          map[string]string{"dex.config": "abcdefg"},
			containsError: "invalid dex.config",
		},
		"General_OIDCConfigured": {
			validator: "general",
			data: map[string]string{
				"url": "https://myargocd.com",
				"oidc.config": `
name: Okta
issuer: https://dev-123456.oktapreview.com
clientID: aaaabbbbccccddddeee
clientSecret: aaaabbbbccccddddeee`,
			},
			containsSummary: "OIDC is configured",
		},
		"General_DexConfiguredMissingURL": {
			validator: "general",
			data: map[string]string{
				"dex.config": `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
			},
			containsSummary: "Dex is configured ('url' field is missing)",
		},
		"Kustomize_ModifiedOptions": {
			validator:       "kustomize",
			containsSummary: "default options",
		},
		"Kustomize_DefaultOptions": {
			validator: "kustomize",
			data: map[string]string{
				"kustomize.buildOptions":  "updated-options (2 versions)",
				"kustomize.versions.v123": "binary-123",
				"kustomize.versions.v321": "binary-321",
			},
			containsSummary: "updated-options",
		},
		"Repositories": {
			validator: "repositories",
			data: map[string]string{
				"repositories": `
- url: https://github.com/argoproj/my-private-repository1
- url: https://github.com/argoproj/my-private-repository2`,
			},
			containsSummary: "2 repositories",
		},
		"Accounts": {
			validator: "accounts",
			data: map[string]string{
				"accounts.user1": "apiKey, login",
				"accounts.user2": "login",
				"accounts.user3": "apiKey",
			},
			containsSummary: "4 accounts",
		},
		"ResourceOverrides": {
			validator: "resource-overrides",
			data: map[string]string{
				"resource.customizations": `
admissionregistration.k8s.io/MutatingWebhookConfiguration:
  ignoreDifferences: |
  jsonPointers:
  - /webhooks/0/clientConfig/caBundle`,
			},
			containsSummary: "2 resource overrides",
		},
	}
	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			validator, ok := validatorsByGroup[tc.validator]
			if !assert.True(t, ok) {
				return
			}
			summary, err := validator(newSettingsManager(tc.data))
			if tc.containsSummary != "" {
				require.NoError(t, err)
				assert.Contains(t, summary, tc.containsSummary)
			} else if tc.containsError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.containsError)
			}
		})
	}
}

const (
	testDeploymentYAML = `apiVersion: v1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 0`
)

const (
	testCustomResourceYAML = `apiVersion: v1
apiVersion: example.com/v1alpha1
kind: ExampleResource
metadata:
  name: example-resource
  labels:
    app: example
spec:
  replicas: 0`
)

const (
	testCronJobYAML = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello
  namespace: test-ns
  uid: "123"
spec:
  schedule: "* * * * *"`
)

func tempFile(content string) (string, io.Closer, error) {
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		return "", nil, err
	}
	_, err = f.Write([]byte(content))
	if err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	defer func() {
		if err = f.Close(); err != nil {
			panic(err)
		}
	}()
	return f.Name(), utils.NewCloser(func() error {
		return os.Remove(f.Name())
	}), nil
}

func TestValidateSettingsCommand_NoErrors(t *testing.T) {
	cmd := NewValidateSettingsCommand(newCmdContext(map[string]string{}))
	out, err := captureStdout(func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	require.NoError(t, err)
	for k := range validatorsByGroup {
		assert.Contains(t, out, fmt.Sprintf("✅ %s", k))
	}
}

func TestResourceOverrideIgnoreDifferences(t *testing.T) {
	f, closer, err := tempFile(testDeploymentYAML)
	require.NoError(t, err)
	defer utils.Close(closer)

	t.Run("NoOverridesConfigured", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"ignore-differences", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Ignore differences are not configured for 'apps/Deployment'\n")
	})

	t.Run("DataIgnored", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `apps/Deployment:
  ignoreDifferences: |
    jsonPointers:
    - /spec`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"ignore-differences", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "< spec:")
	})
}

func TestResourceOverrideHealth(t *testing.T) {
	f, closer, err := tempFile(testCustomResourceYAML)
	require.NoError(t, err)
	defer utils.Close(closer)

	t.Run("NoHealthAssessment", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `example.com/ExampleResource: {}`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"health", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Health script is not configured for 'example.com/ExampleResource'\n")
	})

	t.Run("HealthAssessmentConfigured", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `example.com/ExampleResource:
  health.lua: |
    return { status = "Progressing" }
`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"health", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Progressing")
	})

	t.Run("HealthAssessmentConfiguredWildcard", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `example.com/*:
  health.lua: |
    return { status = "Progressing" }
`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"health", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Progressing")
	})
}

func TestResourceOverrideAction(t *testing.T) {
	f, closer, err := tempFile(testDeploymentYAML)
	require.NoError(t, err)
	defer utils.Close(closer)

	cronJobFile, closer, err := tempFile(testCronJobYAML)
	require.NoError(t, err)
	defer utils.Close(closer)

	t.Run("NoActions", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `apps/Deployment: {}`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"run-action", f, "test"})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Actions are not configured")
	})

	t.Run("OldStyleActionConfigured", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `apps/Deployment:
  actions: |
    discovery.lua: |
      actions = {}
      actions["resume"] = {["disabled"] = false}
      actions["restart"] = {["disabled"] = false}
      return actions
    definitions:
    - name: test
      action.lua: |
        obj.metadata.labels["test"] = 'updated'
        return obj
`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"run-action", f, "test"})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "test: updated")

		out, err = captureStdout(func() {
			cmd.SetArgs([]string{"list-actions", f})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, `NAME     DISABLED
restart  false
resume   false
`)
	})

	t.Run("NewStyleActionConfigured", func(t *testing.T) {
		cmd := NewResourceOverridesCommand(newCmdContext(map[string]string{
			"resource.customizations": `batch/CronJob:
  actions: |
    discovery.lua: |
      actions = {}
      actions["create-a-job"] = {["disabled"] = false}
      return actions
    definitions:
    - name: test
      action.lua: |
        job1 = {}
        job1.apiVersion = "batch/v1"
        job1.kind = "Job"
        job1.metadata = {}
        job1.metadata.name = "hello-1"
        job1.metadata.namespace = "obj.metadata.namespace"
        impactedResource1 = {}
        impactedResource1.operation = "create"
        impactedResource1.resource = job1
        result = {}
        result[1] = impactedResource1
        return result
`,
		}))
		out, err := captureStdout(func() {
			cmd.SetArgs([]string{"run-action", cronJobFile, "test"})
			err := cmd.Execute()
			require.NoError(t, err)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "resource was created:")
		assert.Contains(t, out, "hello-1")

		out, err = captureStdout(func() {
			cmd.SetArgs([]string{"list-actions", cronJobFile})
			err := cmd.Execute()
			require.NoError(t, err)
		})

		require.NoError(t, err)
		assert.Contains(t, out, "NAME")
		assert.Contains(t, out, "DISABLED")
		assert.Contains(t, out, "create-a-job")
		assert.Contains(t, out, "false")
	})
}
