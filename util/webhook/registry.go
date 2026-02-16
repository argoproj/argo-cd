package webhook

import (
	"bytes"
	"fmt"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/glob"
	log "github.com/sirupsen/logrus"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"strings"
)

type WebhookRegistryEvent struct {
	// RegistryURL is the URL of the registry that sent the webhook
	// eg. ghcr.io
	RegistryURL string `json:"registryUrl,omitempty"`
	// Repository is the repository name
	// eg. user/repo
	Repository string `json:"repository,omitempty"`
	// Tag is the image tag
	// eg. 0.3.0
	Tag string `json:"tag,omitempty"`
	// Digest is the content digest of the image (optional)
	Digest string `json:"digest,omitempty"`
}

// RegistryParser parses registry-specific webhook payloads
type RegistryParser interface {
	CanHandle(r *http.Request) bool
	Parse(body []byte) (*WebhookRegistryEvent, error)
}

type WebhookRegistryHandler struct {
	parsers []RegistryParser
	secret  string
}

func NewWebhookRegistryHandler(secret string) *WebhookRegistryHandler {
	return &WebhookRegistryHandler{
		parsers: []RegistryParser{
			NewGHCRParser(),
		},
		secret: secret,
	}
}

func (h *WebhookRegistryHandler) ProcessWebhook(r *http.Request) (*WebhookRegistryEvent, error) {
	fmt.Println("we are processing")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if err := h.validateSignature(r, body); err != nil {
		return nil, err
	}

	fmt.Println("signature validated")
	for _, p := range h.parsers {
		if p.CanHandle(r) {
			return p.Parse(body)
		}
	}

	return nil, fmt.Errorf("unsupported registry webhook")
}

func (a *ArgoCDWebhookHandler) HandleRegistryEvent(event *WebhookRegistryEvent) {
	fmt.Println("time to refresh...")
	// Construct full OCI repo URL used in Argo CD Applications
	repoURL := fmt.Sprintf("oci://%s/%s",
		event.RegistryURL,
		event.Repository,
	)
	revision := event.Tag

	log.WithFields(log.Fields{
		"repo": repoURL,
		"tag":  revision,
	}).Info("Received registry webhook event")

	// Determine namespaces to search
	nsFilter := a.ns
	if len(a.appNs) > 0 {
		nsFilter = ""
	}
	appIf := a.appsLister.Applications(nsFilter)
	apps, err := appIf.List(labels.Everything())
	if err != nil {
		log.Errorf("Failed to list applications: %v", err)
		return
	}

	var filteredApps []v1alpha1.Application
	for _, app := range apps {
		if app.Namespace == a.ns || glob.MatchStringInList(a.appNs, app.Namespace, glob.REGEXP) {
			filteredApps = append(filteredApps, *app)
		}
	}

	for _, app := range filteredApps {
		sources := app.Spec.GetSources()
		if app.Spec.SourceHydrator != nil {
			sources = append(sources, app.Spec.SourceHydrator.GetDrySource())
		}

		for _, source := range sources {
			if normalizeOCI(source.RepoURL) != normalizeOCI(repoURL) {
				fmt.Println("skipping normalizing")
				continue
			}
			if !compareRevisions(revision, source.TargetRevision) {
				fmt.Println("revision not matching, skipping")
				fmt.Println("revision", revision, "targetRevision", source.TargetRevision)
				continue
			}
			log.Infof("Refreshing app '%s' due to OCI push %s:%s",
				app.Name, repoURL, revision,
			)

			namespacedAppInterface :=
				a.appClientset.ArgoprojV1alpha1().
					Applications(app.Namespace)

			if _, err := argo.RefreshApp(
				namespacedAppInterface,
				app.Name,
				v1alpha1.RefreshTypeNormal,
				false,
			); err != nil {
				log.Errorf("Failed to refresh app '%s': %v",
					app.Name, err)
			}

			break // no need to check other sources
		}
	}
}

func normalizeOCI(url string) string {
	return strings.ToLower(strings.TrimSuffix(url, "/"))
}
