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

type GHCRParser struct{}

func NewGHCRParser() *GHCRParser {
	return &GHCRParser{}
}

func (p *GHCRParser) CanHandle(r *http.Request) bool {
	return r.Header.Get("X-GitHub-Event") == "package" || r.Header.Get("X-GitHub-Event") == "registry_package"
}

func (p *GHCRParser) Parse(body []byte) (*WebhookRegistryEvent, error) {
	fmt.Println("we are in parse")
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
						Name string `json:"name"`
					} `json:"tag"`
					Digest string `json:"digest"`
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
	fmt.Println("tag is", tag)
	digest := payload.Package.PackageVersion.ContainerMetadata.Digest

	if tag == "" {
		return nil, fmt.Errorf("missing tag")
	}

	fmt.Println("about to return from parse")

	return &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  repository,
		Tag:         tag,
		Digest:      digest,
	}, nil
}

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
