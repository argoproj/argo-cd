package repository

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
)

// templateFuncMap contains sprig functions plus custom helpers for templating.
// Initialized once for performance.
var templateFuncMap template.FuncMap

// baseGoTemplate is a pre-initialized template with all functions loaded.
// Cloning this is faster than calling Funcs() on a new template each time.
var baseGoTemplate *template.Template

func init() {
	templateFuncMap = sprig.TxtFuncMap()
	// Remove potentially dangerous functions
	delete(templateFuncMap, "env")
	delete(templateFuncMap, "expandenv")
	delete(templateFuncMap, "getHostByName")

	baseGoTemplate = template.New("base").Funcs(templateFuncMap)
}

// HTTPTemplateAuthenticator exchanges identity tokens for credentials using
// configurable HTTP request templates. This allows integration with various
// token exchange endpoints (OAuth 2.0, OIDC, custom APIs) without requiring
// provider-specific code.
//
// Templates use Go template syntax with Sprig functions available.
// Built-in variables: .token, .registry, .repo, plus any custom .params
//
// Example configurations:
//
//	# Octo-STS (GitHub tokens from OIDC)
//	method: GET
//	pathTemplate: "/sts/exchange?scope={{ .repo }}&identity={{ .policy }}"
//	params:
//	  policy: "argocd"
//
//	# ACR token exchange
//	method: POST
//	pathTemplate: "/oauth2/exchange"
//	bodyTemplate: "grant_type=access_token&service={{ .registry }}&access_token={{ .token }}"
//	authType: none
//	responseTokenField: refresh_token
//
//	# JFrog OIDC token exchange
//	method: POST
//	pathTemplate: "/access/api/v1/oidc/token"
//	bodyTemplate: '{"grant_type":"urn:ietf:params:oauth:grant-type:token-exchange","subject_token":"{{ .token }}","provider_name":"{{ .provider }}"}'
//	authType: none
//	params:
//	  provider: "my-oidc-provider"
type HTTPTemplateAuthenticator struct {
	HTTPClient *http.Client
}

// NewHTTPTemplateAuthenticator creates a new template-based HTTP authenticator
func NewHTTPTemplateAuthenticator() *HTTPTemplateAuthenticator {
	return &HTTPTemplateAuthenticator{}
}

// Authenticate exchanges an identity token for registry credentials using HTTP templates
func (a *HTTPTemplateAuthenticator) Authenticate(ctx context.Context, token *Token, repoURL string, config *Config) (*Credentials, error) {
	if token.Type != TokenTypeBearer {
		return nil, fmt.Errorf("http template authenticator requires a bearer token, got %s", token.Type)
	}
	if token.Token == "" {
		return nil, errors.New("empty bearer token")
	}
	if config.PathTemplate == "" {
		return nil, errors.New("pathTemplate is required for HTTP template authenticator")
	}

	// Parse repo URL to extract registry and path
	registry, repoPath, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo URL: %w", err)
	}

	// Build template variables
	vars := map[string]string{
		"token":    token.Token,
		"registry": registry,
		"repo":     repoPath,
	}
	// Add custom params
	for k, v := range config.Params {
		vars[k] = v
	}

	// Determine scheme
	scheme := "https"
	if config.Insecure {
		scheme = "http"
	}

	// Build the full URL
	path, err := substituteTemplate(config.PathTemplate, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render path template: %w", err)
	}

	// Use AuthHost if specified, otherwise use registry from repo URL
	host := registry
	if config.AuthHost != "" {
		host = config.AuthHost
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, host, path)

	// Determine HTTP method
	method := strings.ToUpper(config.Method)
	if method == "" {
		method = http.MethodGet
	}

	// Build request body if template provided
	var bodyReader io.Reader
	var contentType string
	if config.BodyTemplate != "" {
		body, err := substituteTemplate(config.BodyTemplate, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to render body template: %w", err)
		}
		bodyReader = strings.NewReader(body)

		// Auto-detect content type
		trimmed := strings.TrimSpace(config.BodyTemplate)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			contentType = "application/json"
		} else {
			contentType = "application/x-www-form-urlencoded"
		}
	}

	log.WithFields(log.Fields{
		"url":      fullURL,
		"method":   method,
		"authType": config.AuthType,
	}).Info("HTTPTemplate: making token exchange request")

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Set authentication based on authType
	authType := strings.ToLower(config.AuthType)
	switch authType {
	case "basic":
		if config.Username == "" {
			return nil, errors.New("username is required for basic auth")
		}
		req.SetBasicAuth(config.Username, token.Token)
	case "none":
		// Token is only in the template, no Authorization header
	case "bearer", "":
		// Default: Bearer token in header
		req.Header.Set("Authorization", "Bearer "+token.Token)
	default:
		return nil, fmt.Errorf("unknown authType: %s", authType)
	}

	// Execute request
	client := a.getHTTPClient(config.Insecure)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"statusCode": resp.StatusCode,
			"body":       truncateString(string(respBody), 200),
		}).Error("HTTPTemplate: request failed")
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, truncateString(string(respBody), 200))
	}

	// Parse response to extract token
	accessToken, err := extractTokenFromResponse(respBody, config.ResponseTokenField)
	if err != nil {
		return nil, fmt.Errorf("failed to extract token from response: %w", err)
	}

	// Determine username for credentials
	username := config.Username

	log.WithFields(log.Fields{
		"registry": registry,
		"username": username,
	}).Info("HTTPTemplate: successfully obtained credentials")

	return &Credentials{
		Username: username,
		Password: accessToken,
	}, nil
}

// substituteTemplate executes a Go template with the provided variables.
// Templates use Go template syntax: {{ .token }}, {{ .registry | urlquery }}, etc.
// All Sprig functions are available (except env, expandenv, getHostByName).
func substituteTemplate(tmplStr string, vars map[string]string) (string, error) {
	// Clone the base template which has sprig funcs pre-loaded
	cloned, err := baseGoTemplate.Clone()
	if err != nil {
		return "", fmt.Errorf("failed to clone base template: %w", err)
	}

	parsed, err := cloned.Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := parsed.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// extractTokenFromResponse extracts the token from a JSON response
func extractTokenFromResponse(body []byte, fieldName string) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// If specific field requested, use it
	if fieldName != "" {
		if val, ok := data[fieldName]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return strVal, nil
			}
		}
	}

	return "", fmt.Errorf("field '%q' not found or empty in response", fieldName)
}

// parseRepoURL parses a repository URL and extracts the registry host and path.
// e.g., "oci://quay.io/myorg/myrepo" -> ("quay.io", "myorg/myrepo", nil)
func parseRepoURL(repoURL string) (registry, repoPath string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", repoURL, err)
	}

	registry = u.Host
	repoPath = strings.TrimPrefix(u.Path, "/")
	return registry, repoPath, nil
}

func (a *HTTPTemplateAuthenticator) getHTTPClient(insecure bool) *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	client := &http.Client{}
	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return client
}

// truncateString truncates a string to maxLen, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure HTTPTemplateAuthenticator implements Authenticator
var _ Authenticator = (*HTTPTemplateAuthenticator)(nil)
