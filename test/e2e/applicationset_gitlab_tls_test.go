package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets/utils"
)

func TestPullRequestGitLabMutualTLS(t *testing.T) {
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile("../fixture/certs/" + name)
		require.NoError(t, err)
		return data
	}
	ca := read("argocd-test-ca.crt")
	cert, err := tls.X509KeyPair(read("argocd-test-server.crt"), read("argocd-test-server.key"))
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(ca))
	server := testServerWithPort(t, 8350, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/projects/42/merge_requests", r.URL.Path)
		assert.Equal(t, "gitlab-api-token", r.Header.Get("Private-Token"))
		require.NotEmpty(t, r.TLS.VerifiedChains)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[{"iid":7,"title":"Preview","source_branch":"feature","target_branch":"main","sha":"824a5c987fdfb2b0629e9dbf5f31636c69ba4772","author":{"username":"dev"}}]`))
		assert.NoError(t, err)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	ctx := Given(t)
	clients := utils.GetE2EFixtureK8sClient(t).KubeClientset.CoreV1()
	namespace := fixture.TestNamespace()
	secretName, caName := "gitlab-client-tls", "gitlab-server-ca"
	_, err = clients.Secrets(namespace).Create(t.Context(), &corev1.Secret{
		Name: secretName, Namespace: namespace, Type: corev1.SecretTypeTLS,
		Labels: map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeSCMCreds},
		Data:   map[string][]byte{corev1.TLSCertKey: read("argocd-test-client.crt"), corev1.TLSPrivateKeyKey: read("argocd-test-client.key"), "token": []byte("gitlab-api-token")},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, clients.Secrets(namespace).Delete(context.WithoutCancel(t.Context()), secretName, metav1.DeleteOptions{}))
	})
	_, err = clients.ConfigMaps(namespace).Create(t.Context(), &corev1.ConfigMap{Name: caName, Namespace: namespace, Data: map[string]string{"ca.crt": string(ca)}}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, clients.ConfigMaps(namespace).Delete(context.WithoutCancel(t.Context()), caName, metav1.DeleteOptions{}))
	})
	expected := v1alpha1.Application{
		Kind: "Application", APIVersion: "argoproj.io/v1alpha1", Name: "gitlab-preview-7", Namespace: namespace,
		Finalizers: []string{v1alpha1.ResourcesFinalizerName},
		Spec: v1alpha1.ApplicationSpec{
			Project:     "default",
			Source:      &v1alpha1.ApplicationSource{RepoURL: "git@github.com:applicationset-test-org/argocd-example-apps.git", TargetRevision: "824a5c987fdfb2b0629e9dbf5f31636c69ba4772", Path: "kustomize-guestbook"},
			Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "gitlab-preview-7"},
		},
	}
	ctx.When().Create(v1alpha1.ApplicationSet{Spec: v1alpha1.ApplicationSetSpec{
		GoTemplate: true,
		Generators: []v1alpha1.ApplicationSetGenerator{{PullRequest: &v1alpha1.PullRequestGenerator{GitLab: &v1alpha1.PullRequestGeneratorGitLab{
			Project: "42", API: server.URL,
			TokenRef:               &v1alpha1.SecretRef{SecretName: secretName, Key: "token"},
			CARef:                  &v1alpha1.ConfigMapKeyRef{ConfigMapName: caName, Key: "ca.crt"},
			TLSClientCertSecretRef: &corev1.LocalObjectReference{Name: secretName},
		}}}},
		Template: v1alpha1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "gitlab-preview-{{.number}}"},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Source:      &v1alpha1.ApplicationSource{RepoURL: expected.Spec.Source.RepoURL, TargetRevision: "{{.head_sha}}", Path: "kustomize-guestbook"},
				Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "gitlab-preview-{{.number}}"},
			},
		},
	}}).Then().Expect(ApplicationsExist([]v1alpha1.Application{expected}))
}
