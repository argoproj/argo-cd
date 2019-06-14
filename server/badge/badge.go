package badge

import (
	"net/http"

	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
)

func NewHandler(appClientset versioned.Interface, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// h.appClientset.ArgoprojV1alpha1().Applications(h.namespace).Get("123", v1.GetOptions{})
	// Sample url: http://localhost:8080/api/badge?name=123
	_, err := w.Write([]byte("hello world"))
	if err != nil {
		w.WriteHeader(503)
	} else {
		w.WriteHeader(200)
	}
}
