package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	scmmanager "github.com/scm-manager/goscm/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestSCMManagerWebhookAuthentication(t *testing.T) {
	const secret = "test-webhook-secret"
	labels := map[string]string{"app.kubernetes.io/part-of": "argocd"}
	configMap := &corev1.ConfigMap{}
	configMap.Name = common.ArgoCDConfigMapName
	configMap.Namespace = "argocd"
	configMap.Labels = labels
	secretObject := &corev1.Secret{Data: map[string][]byte{
		"server.secretkey":          nil,
		"webhook.scmmanager.secret": []byte(secret),
	}}
	secretObject.Name = common.ArgoCDSecretName
	secretObject.Namespace = "argocd"
	secretObject.Labels = labels
	client := fake.NewClientset(configMap, secretObject)
	config, err := settings.NewSettingsManager(t.Context(), client, "argocd").GetSettings()
	require.NoError(t, err)
	parsers := NewParsers(config, ParserOptions{SCMManagerEvents: []scmmanager.Event{scmmanager.PushEvent}})
	payload, err := os.ReadFile("testdata/scmm-webhook-data.json")
	require.NoError(t, err)
	mac := hmac.New(sha1.New, []byte(secret))
	_, err = mac.Write(payload)
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		signature string
		status    int
	}{
		{name: "unsigned", status: http.StatusUnauthorized},
		{name: "short signature", signature: "x", status: http.StatusUnauthorized},
		{name: "signed", signature: fmt.Sprintf("sha1=%x", mac.Sum(nil)), status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", bytes.NewReader(payload))
			req.Header.Set("X-SCM-Event", "Push")
			if tc.signature != "" {
				req.Header.Set("X-SCM-Signature", tc.signature)
			}
			queue := make(chan any, 1)
			response := httptest.NewRecorder()
			HandleRequest(response, req, parsers, 1<<20, queue)
			assert.Equal(t, tc.status, response.Code)
			if tc.status == http.StatusOK {
				require.Len(t, queue, 1)
				assert.IsType(t, scmmanager.PushEventPayload{}, <-queue)
			} else {
				assert.Empty(t, queue)
			}
		})
	}
}
