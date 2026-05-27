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

// dockerHubWebhookType is the value of the "type" query parameter that identifies
// an incoming request as a Docker Hub webhook. Docker Hub sends no provider-specific
// header, so the user appends this to the configured webhook URL
// (e.g. /api/webhook?type=dockerhub) to route the request to this parser.
const dockerHubWebhookType = "dockerhub"

// dockerhubParser parses webhook payloads sent by Docker Hub.
//
// It extracts image push events from Docker Hub repository webhooks and converts
// them into a normalized RegistryEvent. Docker Hub neither signs its payloads nor
// sends a distinguishing header, so requests are claimed via a query parameter
// (see dockerHubWebhookType) and authenticated with an optional shared secret
// carried in the request URL.
type dockerhubParser struct {
	secret string
}

// DockerHubPayload represents the subset of the webhook payload Docker Hub sends
// for repository push events that we need to identify the pushed image.
// See https://docs.docker.com/docker-hub/webhooks/.
type DockerHubPayload struct {
	Repository struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		RepoName  string `json:"repo_name"`
		Status    string `json:"status"`
	} `json:"repository"`
	PushData struct {
		Tag string `json:"tag"`
	} `json:"push_data"`
}

// newDockerHubParser creates a new dockerhubParser instance.
//
// Docker Hub cannot sign webhook payloads, so the parser authenticates requests
// using a shared secret supplied as a query parameter. If no secret is configured,
// incoming events are accepted without validation and a warning is logged.
func newDockerHubParser(secret string) *dockerhubParser {
	if secret == "" {
		log.Warn("DockerHub webhook secret is not configured; incoming webhook events will not be validated")
	}
	return &dockerhubParser{
		secret: secret,
	}
}

// CanHandle reports whether the HTTP request corresponds to a Docker Hub webhook.
//
// Docker Hub does not set a provider-specific header (unlike GitHub's
// X-GitHub-Event), so the request is identified solely by the "type=dockerhub"
// query parameter the user adds to the configured webhook URL.
func (p *dockerhubParser) CanHandle(r *http.Request) bool {
	return r.URL.Query().Get("type") == dockerHubWebhookType
}

// Parse validates the request and extracts image push details from a Docker Hub
// webhook payload.
//
// It rejects non-POST requests, verifies the shared secret, and returns a
// normalized RegistryEvent containing the registry host ("docker.io"), repository,
// and pushed tag. It returns nil, nil for events that are intentionally skipped
// (a payload missing its repository or tag) and an error only for malformed
// payloads or a failed secret check.
func (p *dockerhubParser) Parse(r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("unexpected method %q for DockerHub webhook", r.Method)
	}
	if err := p.validateSecret(r); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var payload DockerHubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DockerHub webhook payload: %w", err)
	}
	// repo_name is the full "namespace/name". Fall back to assembling it from the
	// individual fields, then canonicalize official images (which arrive without a
	// namespace) to the implicit "library/" namespace so the OCI repo URL matches.
	repository := payload.Repository.RepoName
	if repository == "" && payload.Repository.Name != "" {
		if payload.Repository.Namespace != "" {
			repository = payload.Repository.Namespace + "/" + payload.Repository.Name
		} else {
			repository = payload.Repository.Name
		}
	}
	if repository != "" && !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}

	if repository == "" || payload.PushData.Tag == "" {
		log.Debug("Skipping DockerHub webhook event: missing repository or tag")
		return nil, nil
	}

	return &RegistryEvent{
		RegistryURL: "docker.io",
		Repository:  repository,
		Tag:         payload.PushData.Tag,
	}, nil
}

// validateSecret verifies the shared secret supplied in the "secret" query
// parameter.
//
// Docker Hub does not support signing webhook payloads, so the secret is carried
// in the request URL and compared in constant time. If no secret is configured,
// validation is skipped (the endpoint is open and a warning was logged at
// construction). A mismatch returns ErrHMACVerificationFailed, which the handler
// maps to an HTTP 401 response.
func (p *dockerhubParser) validateSecret(r *http.Request) error {
	if p.secret == "" {
		return nil // open endpoint
	}
	provided := r.URL.Query().Get("secret")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(p.secret)) != 1 {
		return fmt.Errorf("%w: invalid DockerHub webhook secret", ErrHMACVerificationFailed)
	}
	return nil
}
