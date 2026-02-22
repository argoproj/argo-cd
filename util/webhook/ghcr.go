package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GHCRParser parses webhook payloads sent by GitHub Container Registry (GHCR).
//
// It extracts container image publication events from GitHub package webhooks
// and converts them into a normalized WebhookRegistryEvent structure.
type GHCRParser struct{}

// NewGHCRParser creates a new GHCRParser instance.
//
// The parser supports GitHub package webhook events for container images
// published to GitHub Container Registry (ghcr.io).
func NewGHCRParser() *GHCRParser {
	return &GHCRParser{}
}

// Parse extracts container publication details from a GHCR webhook payload.
//
// The method expects a GitHub package event with action "published" for a
// container package. It returns a normalized WebhookRegistryEvent containing
// the registry host, repository, tag, and digest. Non-container packages,
// unsupported actions, or missing tags result in an error.
func (p *GHCRParser) Parse(body []byte) (*WebhookRegistryEvent, error) {
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
		return nil, fmt.Errorf("ignoring action")
	}

	if strings.ToLower(payload.Package.PackageType) != "container" {
		return nil, fmt.Errorf("not a container package")
	}

	repository := payload.Package.Owner.Login + "/" + payload.Package.Name
	tag := payload.Package.PackageVersion.ContainerMetadata.Tag.Name
	digest := payload.Package.PackageVersion.ContainerMetadata.Tag.Digest

	if tag == "" {
		return nil, fmt.Errorf("missing tag")
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
func (h *WebhookRegistryHandler) validateSignature(r *http.Request, body []byte) error {
	if h.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			return fmt.Errorf("missing signature")
		}

		mac := hmac.New(sha256.New, []byte(h.secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return fmt.Errorf("invalid signature")
		}
	}

	return nil
}
