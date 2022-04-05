package extensions

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/rbac"
)

type item struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Object   string `json:"object"`
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

const labelKey = "argocd.argoproj.io/extension"

func NewHandler(ctx context.Context, secrets v1.SecretInterface, apps v1alpha1.ApplicationInterface, enforcer *rbac.Enforcer) (http.Handler, error) {

	log.Info("Loading API extensions")

	items, err := secrets.List(ctx, metav1.ListOptions{LabelSelector: labelKey})
	if err != nil {
		return nil, err
	}

	// a short-lived cache for looking up the project for an application
	projectCache := gocache.New(time.Minute, time.Minute)

	appProject := func(ctx context.Context, appName string) (string, error) {
		if project, ok := projectCache.Get(appName); ok {
			return project.(string), nil
		}
		app, err := apps.Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		project := app.Spec.GetProject()
		projectCache.Set(appName, project, time.Minute)
		return project, nil
	}

	// for applications, the RBAC object is a special case, it is "project/appName"
	rbacObject := func(ctx context.Context, resource, object string) (string, error) {
		if resource == rbacpolicy.ResourceApplications {
			project, err := appProject(ctx, object)
			return fmt.Sprintf("%s/%s", project, object), err
		}
		return object, nil
	}
	r := mux.NewRouter()

	for _, i := range items.Items {

		name := i.Labels[labelKey]
		url := string(i.Data["url"])
		headers := http.Header{}
		paths := map[string]map[string]item{}

		if err := yaml.UnmarshalStrict(i.Data["headers"], &headers); err != nil {
			return nil, err
		}
		if err := yaml.UnmarshalStrict(i.Data["paths"], &paths); err != nil {
			return nil, err
		}

		basePath := fmt.Sprintf("/api/extensions/%s", name)

		// prevent a UI extension accidentally DoS the Argo CD Server
		// one rate-limiter per extension, means one bad extension does not impact others
		limiter := rate.NewLimiter(1, 3)

		for subPath, methods := range paths {
			path := basePath + subPath
			log.
				WithField("name", name).
				WithField("path", path).
				Info("Loading API extension")

			r.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()

				// rate-limit the request
				if !limiter.Allow() {
					http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
					return
				}

				// get the claims from the context
				claims, ok := ctx.Value("claims").(*jwt.MapClaims)
				if !ok {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				// is the method allowed?
				m, ok := methods[r.Method]
				if !ok {
					http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
					return
				}

				// get the object from the request
				vars := mux.Vars(r)
				object, ok := vars[m.Object]
				if !ok {
					http.Error(w, http.StatusText(http.StatusBadRequest)+": missing object", http.StatusBadRequest)
					return
				}

				// create a conventional name for the object
				object, err := rbacObject(ctx, m.Resource, object)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusServiceUnavailable)+": object lookup failed", http.StatusServiceUnavailable)
					return
				}

				// enforce access controls
				if err := enforcer.EnforceErr(claims, m.Resource, m.Action, object); err != nil {
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}

				// proxy the request
				req, err := http.NewRequestWithContext(
					// use the HTTP request's context, if that is cancelled, this will be canceled to
					ctx,
					r.Method,
					url+strings.TrimPrefix(r.URL.Path, basePath),
					r.Body,
				)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				req.Header = headers.Clone()

				// we need to provide information to the downstream about who is making the request
				req.Header.Add("claims-sub", (*claims)["sub"].(string))

				log.WithField("sub", req.Header.Get("claims-sub")).
					WithField("method", req.Method).
					WithField("url", req.URL).
					Info("Executing API extension request")

				resp, err := httpClient.Do(req)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()

				w.WriteHeader(resp.StatusCode)
				for k, v := range resp.Header {
					w.Header()[k] = v
				}
				_, _ = io.Copy(w, resp.Body)
			})
		}
	}

	return r, nil
}
