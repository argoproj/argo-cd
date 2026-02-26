package webhook

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/glob"

	"k8s.io/apimachinery/pkg/labels"
)

// RegistryEvent represents a normalized container registry webhook event.
//
// It captures the essential information needed to identify an OCI artifact
// update, including the registry host, repository name, tag, and optional
// content digest. This structure is produced by registry-specific parsers
// and consumed by the registry webhook handler to trigger application refreshes.
type RegistryEvent struct {
	// RegistryURL is the hostname of the registry, without protocol or trailing slash.
	// e.g. "ghcr.io", "docker.io", "123456789.dkr.ecr.us-east-1.amazonaws.com"
	// Together with Repository, it forms the OCI repo URL: oci://RegistryURL/Repository.
	// Parsers must ensure this value is consistent with how users configure repoURL
	// in their Argo CD Applications (e.g. oci://ghcr.io/owner/repo).
	RegistryURL string `json:"registryUrl,omitempty"`
	// Repository is the full repository path within the registry, without a leading slash.
	// e.g. "owner/repo" for ghcr.io, "library/nginx" for docker.io.
	// Together with RegistryURL, it forms the OCI repo URL: oci://RegistryURL/Repository.
	Repository string `json:"repository,omitempty"`
	// Tag is the image tag
	// eg. 0.3.0
	Tag string `json:"tag,omitempty"`
	// Digest is the content digest of the image (optional)
	Digest string `json:"digest,omitempty"`
}

// OCIRepoURL returns the full OCI repository URL for use in Argo CD Application
// source matching, e.g. "oci://ghcr.io/owner/repo".
func (e *RegistryEvent) OCIRepoURL() string {
	return fmt.Sprintf("oci://%s/%s", e.RegistryURL, e.Repository)
}

// ErrHMACVerificationFailed is returned when a registry webhook signature check fails.
var ErrHMACVerificationFailed = errors.New("HMAC verification failed")

// RegistryParser defines an interface for parsing registry-specific webhook payloads.
//
// Implementations detect whether they can handle an incoming HTTP request and,
// if so, extract relevant event information into a WebhookRegistryEvent.
// This allows the handler to support multiple container registries via pluggable parsers.
type RegistryParser interface {
	CanHandle(r *http.Request) bool
	Parse(r *http.Request, body []byte) (*RegistryEvent, error)
}

// RegistryHandler processes container registry webhook requests.
//
// It selects the appropriate parser based on the request and delegates
// both signature validation and payload parsing to it.
// The handler supports multiple registry formats through a list of RegistryParsers.
type RegistryHandler struct {
	parsers []RegistryParser
}

// NewWebhookRegistryHandler creates a new WebhookRegistryHandler.
//
// The provided secret is passed to each registry-specific parser, which is
// responsible for its own signature validation. The handler is initialized
// with built-in registry parsers (e.g., GHCR) but can be extended to support
// additional registries.
func NewWebhookRegistryHandler(secret string) *RegistryHandler {
	return &RegistryHandler{
		parsers: []RegistryParser{
			NewGHCRParser(secret),
		},
	}
}

// findParser returns the first parser that can handle the request, or nil.
func (h *RegistryHandler) findParser(r *http.Request) RegistryParser {
	for _, p := range h.parsers {
		if p.CanHandle(r) {
			return p
		}
	}
	return nil
}

// CanHandle reports whether any registered parser can handle the request.
// Used by the top-level handler to route registry webhook requests.
func (h *RegistryHandler) CanHandle(r *http.Request) bool {
	return h.findParser(r) != nil
}

// ProcessWebhook reads the request body and delegates to the first parser
// that can handle the request. Signature validation is handled by each parser.
// Returns nil, nil if the event should be skipped.
func (h *RegistryHandler) ProcessWebhook(r *http.Request) (*RegistryEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	p := h.findParser(r)
	// No parser matched; unsupported registry
	if p == nil {
		return nil, nil
	}
	return p.Parse(r, body)
}

// HandleRegistryEvent processes a normalized registry event and refreshes
// matching Argo CD Applications.
//
// It constructs the full OCI repository URL from the event, finds Applications
// whose sources reference that repository and revision, and triggers a refresh
// for each matching Application. Namespace filters are applied according to the
// handler configuration.
func (a *ArgoCDWebhookHandler) HandleRegistryEvent(event *RegistryEvent) {
	repoURL := event.OCIRepoURL()
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
				log.WithFields(log.Fields{
					"sourceRepoURL": source.RepoURL,
					"eventRepoURL":  repoURL,
				}).Debug("Skipping app: OCI repository URLs do not match")
				continue
			}
			if !compareRevisions(revision, source.TargetRevision) {
				log.WithFields(log.Fields{
					"revision":       revision,
					"targetRevision": source.TargetRevision,
				}).Debug("Skipping app: revision does not match targetRevision")
				continue
			}
			log.Infof("Refreshing app '%s' due to OCI push %s:%s",
				app.Name, repoURL, revision,
			)

			namespacedAppInterface := a.appClientset.ArgoprojV1alpha1().
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
// It removes the oci:// prefix, converts to lowercase, and removes any
// trailing slash to ensure consistent matching between webhook events
// and Application source URLs.
func normalizeOCI(url string) string {
	url = strings.TrimPrefix(url, "oci://")
	url = strings.TrimSuffix(url, "/")
	return strings.ToLower(url)
}
