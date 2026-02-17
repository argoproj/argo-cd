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

// WebhookRegistryEvent represents a normalized container registry webhook event.
//
// It captures the essential information needed to identify an OCI artifact
// update, including the registry host, repository name, tag, and optional
// content digest. This structure is produced by registry-specific parsers
// and consumed by the registry webhook handler to trigger application refreshes.
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

// RegistryParser defines an interface for parsing registry-specific webhook payloads.
//
// Implementations detect whether they can handle an incoming HTTP request and,
// if so, extract relevant event information into a WebhookRegistryEvent.
// This allows the handler to support multiple container registries via pluggable parsers.
type RegistryParser interface {
	CanHandle(r *http.Request) bool
	Parse(body []byte) (*WebhookRegistryEvent, error)
}

// WebhookRegistryHandler processes container registry webhook requests.
//
// It validates the webhook signature, selects an appropriate parser based on
// the request, and converts the payload into a normalized WebhookRegistryEvent.
// The handler supports multiple registry formats through a list of RegistryParsers.
type WebhookRegistryHandler struct {
	parsers []RegistryParser
	secret  string
}

// NewWebhookRegistryHandler creates a new WebhookRegistryHandler.
//
// The provided secret is used to validate webhook signatures. The handler
// is initialized with built-in registry parsers (e.g., GHCR) but can be
// extended to support additional registries.
func NewWebhookRegistryHandler(secret string) *WebhookRegistryHandler {
	return &WebhookRegistryHandler{
		parsers: []RegistryParser{
			NewGHCRParser(),
		},
		secret: secret,
	}
}

// CanHandle reports whether the HTTP request corresponds to a GHCR webhook.
//
// It checks the GitHub event header and returns true for package-related
// events that may contain container registry updates.
func (p *GHCRParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-GitHub-Event") == "package"
}

// ProcessWebhook validates and parses an incoming registry webhook request.
//
// It reads the request body, verifies the webhook signature using the configured
// secret, and delegates parsing to the first RegistryParser that reports it can
// handle the request. On success, it returns a normalized WebhookRegistryEvent.
// An error is returned if signature validation fails or no parser supports the event.
func (h *WebhookRegistryHandler) ProcessWebhook(r *http.Request) (*WebhookRegistryEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if err := h.validateSignature(r, body); err != nil {
		return nil, err
	}

	for _, p := range h.parsers {
		if p.CanHandle(r) {
			return p.Parse(body)
		}
	}

	return nil, fmt.Errorf("unsupported registry webhook")
}

// IsRegistryEvent reports whether the HTTP request corresponds to a supported
// container registry event.
//
// The decision is based on registry-specific headers (e.g., GitHub package).
// It returns true if the request should be handled
// by the registry webhook pipeline.
func IsRegistryEvent(r *http.Request) bool {
	// TODO: add more supported oci-compliant registry type events
	return r.Header.Get("X-GitHub-Event") == "package"
}

// HandleRegistryEvent processes a normalized registry event and refreshes
// matching Argo CD Applications.
//
// It constructs the full OCI repository URL from the event, finds Applications
// whose sources reference that repository and revision, and triggers a refresh
// for each matching Application. Namespace filters are applied according to the
// handler configuration.
func (a *ArgoCDWebhookHandler) HandleRegistryEvent(event *WebhookRegistryEvent) {
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

// normalizeOCI normalizes an OCI repository URL for comparison.
//
// It converts the URL to lowercase and removes any trailing slash to ensure
// consistent matching between webhook events and Application source URLs.
func normalizeOCI(url string) string {
	return strings.ToLower(strings.TrimSuffix(url, "/"))
}
