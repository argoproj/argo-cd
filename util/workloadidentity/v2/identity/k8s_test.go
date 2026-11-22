package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const projectSAUser = "system:serviceaccount:argocd:argocd-project-default"

// fakeAPIServer serves the pod, TokenRequest and SelfSubjectReview calls
// GetToken makes. A real clientset is needed: the fake one has no RESTClient.
type fakeAPIServer struct {
	podSA        string
	reviewStatus int    // status for SelfSubjectReview
	reviewUser   string // username returned on 201
	tokenRequest *authv1.TokenRequest
	reviewAuth   string
}

func (f *fakeAPIServer) clientset(t *testing.T) kubernetes.Interface {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/namespaces/argocd/pods/argocd-pod", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, &corev1.Pod{
			APIVersion: "v1", Kind: "Pod",
			Name: "argocd-pod", Namespace: "argocd", UID: "pod-uid",
			Spec: corev1.PodSpec{ServiceAccountName: f.podSA},
		})
	})
	mux.HandleFunc("POST /api/v1/namespaces/argocd/serviceaccounts/argocd-project-default/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenRequest = &authv1.TokenRequest{}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(f.tokenRequest))
		writeJSON(t, w, http.StatusCreated, &authv1.TokenRequest{
			APIVersion: "authentication.k8s.io/v1", Kind: "TokenRequest",
			Status: authv1.TokenRequestStatus{Token: "minted-jwt"},
		})
	})
	mux.HandleFunc("POST /apis/authentication.k8s.io/v1/selfsubjectreviews", func(w http.ResponseWriter, r *http.Request) {
		f.reviewAuth = r.Header.Get("Authorization")
		if f.reviewStatus != http.StatusCreated {
			writeJSON(t, w, f.reviewStatus, &metav1.Status{
				APIVersion: "v1", Kind: "Status",
				Status: metav1.StatusFailure,
				Code:   int32(f.reviewStatus),
				Reason: map[int]metav1.StatusReason{401: metav1.StatusReasonUnauthorized, 403: metav1.StatusReasonForbidden}[f.reviewStatus],
			})
			return
		}
		writeJSON(t, w, http.StatusCreated, &authv1.SelfSubjectReview{
			APIVersion: "authentication.k8s.io/v1", Kind: "SelfSubjectReview",
			Status: authv1.SelfSubjectReviewStatus{UserInfo: authv1.UserInfo{Username: f.reviewUser}},
		})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	clientset, err := kubernetes.NewForConfig(&rest.Config{
		Host:        server.URL,
		BearerToken: "controller-token",
		ContentType: "application/json",
	})
	require.NoError(t, err)
	return clientset
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, obj any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(obj))
}

func TestK8sProvider_GetToken_BoundObjectRef(t *testing.T) {
	tests := []struct {
		name      string
		podSA     string
		wantBound bool
	}{
		{name: "pod runs as project SA", podSA: "argocd-project-default", wantBound: true},
		// kube-apiserver rejects binding a token to a pod running as a different SA.
		{name: "pod runs as different SA", podSA: "argocd-application-controller", wantBound: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOSTNAME", "argocd-pod")
			api := &fakeAPIServer{podSA: tt.podSA, reviewStatus: http.StatusUnauthorized}
			p := NewK8sProvider(api.clientset(t), "argocd", "argocd-project-default")

			token, err := p.GetToken(t.Context(), "", "")
			require.NoError(t, err)
			assert.Equal(t, "minted-jwt", token.Token)
			require.NotNil(t, api.tokenRequest)
			if tt.wantBound {
				require.NotNil(t, api.tokenRequest.Spec.BoundObjectRef)
				assert.Equal(t, "argocd-pod", api.tokenRequest.Spec.BoundObjectRef.Name)
			} else {
				assert.Nil(t, api.tokenRequest.Spec.BoundObjectRef)
			}
		})
	}
}

func TestK8sProvider_GetToken_RejectsAPIServerTokens(t *testing.T) {
	tests := []struct {
		name         string
		reviewStatus int
		reviewUser   string
		wantErr      string
	}{
		{name: "API server rejects token", reviewStatus: http.StatusUnauthorized},
		{name: "API server accepts token", reviewStatus: http.StatusCreated, reviewUser: projectSAUser, wantErr: "accepted by the Kubernetes API server"},
		{name: "authenticated but forbidden", reviewStatus: http.StatusForbidden, wantErr: "accepted by the Kubernetes API server"},
		// Client-cert credentials authenticate first; the review can't judge the token.
		{name: "review answered for another identity", reviewStatus: http.StatusCreated, reviewUser: "kube-admin"},
		{name: "review fails", reviewStatus: http.StatusInternalServerError, wantErr: "failed to check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOSTNAME", "")
			api := &fakeAPIServer{reviewStatus: tt.reviewStatus, reviewUser: tt.reviewUser}
			p := NewK8sProvider(api.clientset(t), "argocd", "argocd-project-default")

			token, err := p.GetToken(t.Context(), "registry.example.com", "")
			assert.Equal(t, "Bearer minted-jwt", api.reviewAuth, "review must authenticate as the minted token")
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, token)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "minted-jwt", token.Token)
		})
	}
}
