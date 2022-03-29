package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type extension struct {
	url     string
	headers http.Header
}

type extensions map[string]extension

type errorResponse struct {
	Message string `json:"message"`
}

var httpClient = &http.Client{}

const labelKey = "argocd.argoproj.io/extension"

func NewHandler(ctx context.Context, kubernetesClient kubernetes.Interface, namespace string) (http.Handler, error) {

	items, err := kubernetesClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelKey})
	if err != nil {
		return nil, err
	}

	extensions := extensions{}
	for _, i := range items.Items {
		name := i.Labels[labelKey]
		log.WithField("name", name).Info("loading v2 extension")

		e := extension{url: string(i.Data["url"]), headers: http.Header{}}

		for k, v := range i.Data {
			if strings.HasPrefix(k, "header.") {
				e.headers.Add(strings.TrimPrefix(k, "header."), string(v))
			}
		}

		extensions[name] = e
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		name := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/extensions/"), "/")[0]
		e, ok := extensions[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(errorResponse{Message: fmt.Sprintf("extension %s not found", name)})
			return
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, e.url+r.URL.Path, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Message: err.Error()})
			return
		}
		for k, v := range e.headers {
			req.Header[k] = v
		}

		// we need to provide information to the downstream about who is making the request
		if claims, ok := ctx.Value("claims").(*jwt.MapClaims); ok {
			req.Header.Add("claims-sub", (*claims)["sub"].(string))
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(errorResponse{Message: err.Error()})
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}), nil
}
