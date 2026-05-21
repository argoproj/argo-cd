package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	// harborEventPushArtifact is the Harbor webhook event type for artifact push.
	harborEventPushArtifact = "PUSH_ARTIFACT"
)

// HarborPayload represents the Default format payload sent by Harbor webhooks (Harbor 2.x).
// See https://goharbor.io/docs/2.12.0/working-with-projects/project-configuration/configure-webhooks/
type HarborPayload struct {
	// Type is the event type, e.g. "PUSH_ARTIFACT", "PULL_ARTIFACT", "DELETE_ARTIFACT".
	Type      string          `json:"type"`
	OccurAt   int64           `json:"occur_at"`
	Operator  string          `json:"operator"`
	EventData HarborEventData `json:"event_data"`
}

// HarborEventData holds the resources and repository information from a Harbor webhook event.
type HarborEventData struct {
	Resources  []HarborResource `json:"resources"`
	Repository HarborRepository `json:"repository"`
}

// HarborResource describes a single artifact resource within a Harbor webhook event.
type HarborResource struct {
	Digest      string `json:"digest"`
	Tag         string `json:"tag"`
	ResourceURL string `json:"resource_url"`
}

// HarborRepository describes the repository associated with a Harbor webhook event.
type HarborRepository struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	RepoFullName string `json:"repo_full_name"`
	RepoType     string `json:"repo_type"`
	DateCreated  int64  `json:"date_created"`
}

// harborParser parses Harbor registry webhook requests and implements Extractor.
//
// Harbor does not provide a standard signature header; instead it supports a
// configurable Authorization header value that the operator sets in both Harbor's
// webhook configuration and in the ArgoCD secret as webhook.harbor.secret.
// Configuring the secret is required — an empty secret disables Harbor webhook
// support entirely to prevent unauthenticated triggering.
type harborParser struct {
	secret string
}

// newHarborParser constructs a new harborParser with the given pre-shared secret.
// When secret is empty the parser will refuse all requests via CanHandle.
func newHarborParser(secret string) *harborParser {
	return &harborParser{secret: secret}
}

// CanHandle returns true when the request carries the configured Harbor Authorization
// header value. If no secret is configured this always returns false, disabling Harbor
// webhook support.
func (p *harborParser) CanHandle(r *http.Request) bool {
	if p.secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(p.secret)) == 1
}

// Parse reads the request body, validates that it is a Harbor push event,
// and returns the corresponding *RegistryEvent.
// Returns (nil, nil) for non-push events (e.g. pull, delete, scan).
func (p *harborParser) Parse(r *http.Request) (any, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("harbor webhook: failed to read request body: %w", err)
	}

	var payload HarborPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("harbor webhook: failed to parse payload: %w", err)
	}

	if payload.Type != harborEventPushArtifact {
		return nil, nil
	}

	// Find the first resource that carries a tag (digest-only pushes are skipped).
	for _, resource := range payload.EventData.Resources {
		if resource.Tag == "" || resource.ResourceURL == "" {
			continue
		}
		registryURL, err := parseHarborRegistryURL(resource.ResourceURL)
		if err != nil {
			log.Warnf("harbor webhook: skipping resource with unparseable resource_url %q: %v", resource.ResourceURL, err)
			continue
		}
		repository := payload.EventData.Repository.RepoFullName
		if repository == "" {
			// Fall back to constructing namespace/name if repo_full_name is absent.
			ns := payload.EventData.Repository.Namespace
			name := payload.EventData.Repository.Name
			if ns == "" || name == "" {
				return nil, fmt.Errorf("harbor webhook: repository name cannot be determined (repo_full_name, namespace, and name are all empty)")
			}
			repository = ns + "/" + name
		}
		return &RegistryEvent{
			RegistryURL: registryURL,
			Repository:  repository,
			Tag:         resource.Tag,
		}, nil
	}

	return nil, nil
}

// parseHarborRegistryURL extracts the registry hostname from a Harbor resource URL.
// resource_url format: "<hostname>/<namespace>/<repo>:<tag>" or "<hostname>/<namespace>/<repo>@<digest>"
//
// net/url is intentionally avoided here: Harbor's resource_url frequently omits
// the scheme (e.g. "hub.harbor.com/project/repo:tag"), which causes url.Parse to
// treat the entire string as a path rather than a host. Working around that would
// require prepending a dummy scheme and adding extra validation, making the code
// less readable than the simple strings.
func parseHarborRegistryURL(resourceURL string) (string, error) {
	// Strip any scheme that may have been prepended.
	u := resourceURL
	if i := strings.Index(u, "://"); i != -1 {
		u = u[i+3:]
	}
	before, _, ok := strings.Cut(u, "/")
	if !ok {
		return "", fmt.Errorf("harbor webhook: cannot parse registry hostname from resource_url %q", resourceURL)
	}
	return before, nil
}
