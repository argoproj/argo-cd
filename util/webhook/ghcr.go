package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// NewGHCRParser creates a new GHCRParser instance.
//
// The parser supports GitHub package webhook events for container images
// published to GitHub Container Registry (ghcr.io).
func NewGHCRParser(secret string) *GHCRParser {
	return &GHCRParser{secret: secret}
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
func (p *GHCRParser) Parse(r *http.Request, body []byte) (*WebhookRegistryEvent, error) {
	if err := p.validateSignature(r, body); err != nil {
		return nil, err
	}
	var payload struct {
		Action  string `json:"action"`
		Package struct {
			Name        string `json:"name"`
			PackageType string `json:"package_type"`

			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`

			PackageVersion struct {
				ContainerMetadata struct {
					Tag struct {
						Name   string `json:"name"`
						Digest string `json:"digest"`
					} `json:"tag"`
				} `json:"container_metadata"`
			} `json:"package_version"`
		} `json:"package"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
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
	digest := payload.Package.PackageVersion.ContainerMetadata.Tag.Digest

	if tag == "" {
		log.Debugf("Skipping GHCR webhook event: missing tag for repository %q", repository)
		return nil, nil
	}

	return &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  repository,
		Tag:         tag,
		Digest:      digest,
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
