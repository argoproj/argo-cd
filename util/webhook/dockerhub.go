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
// (see dockerHubWebhookType) and authenticated with a shared secret carried in the
// Authorization header or, failing that, the request URL.
//
// Configuring the secret is required: an empty secret disables Docker Hub webhook
// support entirely rather than leaving an unauthenticated endpoint that anyone can
// flood with refresh-triggering requests.
type dockerhubParser struct {
	secret string
}

// dockerHubPayload represents the subset of the webhook payload Docker Hub sends
// for repository push events that we need to identify the pushed image.
// See https://docs.docker.com/docker-hub/webhooks/.
type dockerHubPayload struct {
	Repository struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		RepoName  string `json:"repo_name"`
	} `json:"repository"`
	PushData struct {
		Tag string `json:"tag"`
	} `json:"push_data"`
}

// newDockerHubParser creates a new dockerhubParser instance.
//
// Docker Hub cannot sign webhook payloads, so the parser authenticates requests
// using a shared secret supplied in the Authorization header or as a query
// parameter. When secret is empty the parser refuses all requests via CanHandle,
// which disables Docker Hub webhook support.
func newDockerHubParser(secret string) *dockerhubParser {
	if secret == "" {
		log.Warn("DockerHub webhook secret is not configured; DockerHub webhook support is disabled")
	}
	return &dockerhubParser{
		secret: secret,
	}
}

// CanHandle reports whether the HTTP request corresponds to a Docker Hub webhook.
//
// Docker Hub does not set a provider-specific header (unlike GitHub's
// X-GitHub-Event), so the request is identified by the "type=dockerhub" query
// parameter the user adds to the configured webhook URL. If no secret is
// configured this always returns false, disabling Docker Hub webhook support so
// that the endpoint is never reachable without authentication.
func (p *dockerhubParser) CanHandle(r *http.Request) bool {
	if p.secret == "" {
		return false
	}
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
	// Log the payload size only, never the body itself: it is request-controlled
	// and a misrouted request could carry sensitive data we don't want in logs.
	log.WithField("bytes", len(body)).Debug("Parsing DockerHub webhook payload")
	var payload dockerHubPayload
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
		log.WithField("repository", repository).Debug("Canonicalizing official DockerHub image to library/ namespace")
		repository = "library/" + repository
	}

	if repository == "" || payload.PushData.Tag == "" {
		log.WithFields(log.Fields{
			"repository": repository,
			"tag":        payload.PushData.Tag,
		}).Debug("Skipping DockerHub webhook event: missing repository or tag")
		return nil, nil
	}

	log.WithFields(log.Fields{
		"registry":   "docker.io",
		"repository": repository,
		"tag":        payload.PushData.Tag,
	}).Info("Parsed DockerHub webhook push event")

	return &RegistryEvent{
		RegistryURL: "docker.io",
		Repository:  repository,
		Tag:         payload.PushData.Tag,
	}, nil
}

// validateSecret verifies the shared secret carried by the request.
//
// Docker Hub does not support signing webhook payloads, so the secret is compared
// in constant time against one of two sources, in order of preference:
//
//  1. The Authorization header, matching how the Harbor parser authenticates. Docker
//     Hub itself cannot set custom headers, but this lets operators front Argo CD with
//     a proxy or gateway that moves the secret out of the URL.
//  2. The "secret" query parameter, which is all Docker Hub can supply on its own.
//
// A mismatch returns ErrSecretVerificationFailed, which the handler maps to an HTTP
// 401 response. The error is deliberately not an HMAC error: no signature is involved
// here. An unconfigured secret cannot reach this point, because CanHandle already
// refuses every request in that case.
func (p *dockerhubParser) validateSecret(r *http.Request) error {
	if p.secret == "" {
		return fmt.Errorf("%w: DockerHub webhook secret is not configured", ErrSecretVerificationFailed)
	}
	provided := r.Header.Get("Authorization")
	source := "header"
	if provided == "" {
		provided = r.URL.Query().Get("secret")
		source = "query"
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(p.secret)) != 1 {
		// Never log the secret values; logging whether one was supplied, and where it
		// came from, is enough to distinguish "no secret sent" from "wrong value".
		log.WithFields(log.Fields{
			"secretProvided": provided != "",
			"secretSource":   source,
		}).Debug("DockerHub webhook secret validation failed")
		return fmt.Errorf("%w: invalid DockerHub webhook secret", ErrSecretVerificationFailed)
	}
	return nil
}
