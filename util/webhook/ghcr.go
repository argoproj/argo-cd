package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// GHCRParser parses webhook payloads sent by GitHub Container Registry (GHCR).
//
// It extracts container image publication events from GitHub package webhooks
// and converts them into a normalized WebhookRegistryEvent structure.
type GHCRParser struct {
	secret string
}

// GHCRPayload represents the webhook payload sent by GitHub for
// package events.
type GHCRPayload struct {
	Action  string `json:"action"`
	Package struct {
		Name        string `json:"name"`
		PackageType string `json:"package_type"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
		PackageVersion struct {
			ContainerMetadata struct {
				Tag struct {
					Name string `json:"name"`
				} `json:"tag"`
			} `json:"container_metadata"`
		} `json:"package_version"`
	} `json:"package"`
}

// NewGHCRParser creates a new GHCRParser instance.
//
// The parser supports GitHub package webhook events for container images
// published to GitHub Container Registry (ghcr.io).
func NewGHCRParser(secret string) *GHCRParser {
	if secret == "" {
		log.Warn("GHCR webhook secret is not configured; incoming webhook events will not be validated")
	}
	return &GHCRParser{secret: secret}
}

// ProcessWebhook reads the request body and parses the GHCR webhook payload.
// Returns nil, nil for events that should be skipped.
func (p *GHCRParser) ProcessWebhook(r *http.Request) (*RegistryEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return p.Parse(r, body)
}

// CanHandle reports whether the HTTP request corresponds to a GHCR webhook.
//
// It checks the GitHub event header and returns true for package-related
// events that may contain container registry updates.
func (p *GHCRParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-GitHub-Event") == "package"
}

// Parse validates the request signature and extracts container publication
// details from a GHCR webhook payload.
//
// The method expects a GitHub package event with action "published" for a
// container package. It returns a normalized WebhookRegistryEvent containing
// the registry host, repository, tag, and digest. Returns nil, nil for events
// that are intentionally skipped (unsupported actions, non-container packages,
// or missing tags). Only returns an error for genuinely malformed payloads or
// signature verification failures.
func (p *GHCRParser) Parse(r *http.Request, body []byte) (*RegistryEvent, error) {
	if err := p.validateSignature(r, body); err != nil {
		return nil, err
	}
	var payload GHCRPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GHCR webhook payload: %w", err)
	}

	if payload.Action != "published" {
		log.Debugf("Skipping GHCR webhook event: unsupported action %q", payload.Action)
		return nil, nil
	}

	if !strings.EqualFold(payload.Package.PackageType, "container") {
		log.Debugf("Skipping GHCR webhook event: unsupported package type %q", payload.Package.PackageType)
		return nil, nil
	}

	repository := payload.Package.Owner.Login + "/" + payload.Package.Name
	tag := payload.Package.PackageVersion.ContainerMetadata.Tag.Name

	if tag == "" {
		log.Debugf("Skipping GHCR webhook event: missing tag for repository %q", repository)
		return nil, nil
	}

	return &RegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  repository,
		Tag:         tag,
	}, nil
}

// validateSignature verifies the webhook request signature using HMAC-SHA256.
//
// If a secret is configured, the method checks the X-Hub-Signature-256 header
// against the computed signature of the request body. An error is returned if
// the signature is missing or does not match. If no secret is configured,
// validation is skipped.
func (p *GHCRParser) validateSignature(r *http.Request, body []byte) error {
	if p.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			return fmt.Errorf("%w: missing X-Hub-Signature-256 header", ErrHMACVerificationFailed)
		}

		mac := hmac.New(sha256.New, []byte(p.secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return fmt.Errorf("%w: signature mismatch", ErrHMACVerificationFailed)
		}
	}

	return nil
}
