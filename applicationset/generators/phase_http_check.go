package generators

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func (p *PhaseDeploymentProcessor) runHTTPCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck) error {
	// Validate inputs
	if appSet == nil {
		return errors.New("applicationSet cannot be nil")
	}
	if check.HTTP == nil {
		return errors.New("http check requires http field")
	}

	httpCheck := check.HTTP
	if httpCheck.URL == "" {
		return errors.New("http check requires URL")
	}

	// Security validation for HTTP requests
	if err := validateHTTPSecurity(httpCheck); err != nil {
		log.WithError(err).Error("HTTP check security validation failed")
		return fmt.Errorf("HTTP check security validation failed: %w", err)
	}

	method := httpCheck.Method
	if method == "" {
		method = DefaultHTTPMethod
	}

	expectedStatus := httpCheck.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = DefaultHTTPExpectedStatus
	}
	logger := log.WithFields(log.Fields{
		"check":          check.Name,
		"url":            httpCheck.URL,
		"method":         method,
		"expectedStatus": expectedStatus,
	})

	// Create HTTP client with timeout from context
	client := &http.Client{
		Timeout: 30 * time.Second, // Default client timeout as fallback
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: httpCheck.InsecureSkipVerify,
			},
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	var body io.Reader
	if httpCheck.Body != "" {
		body = strings.NewReader(httpCheck.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, httpCheck.URL, body)
	if err != nil {
		logger.WithError(err).Error("Failed to create HTTP request")
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range httpCheck.Headers {
		req.Header.Set(key, value)
	}

	req.Header.Set("User-Agent", "ArgoCD-ApplicationSet-PhaseCheck/1.0")

	envHeaders := map[string]string{
		"X-AppSet-Name":      appSet.Name,
		"X-AppSet-Namespace": appSet.Namespace,
		"X-Check-Name":       check.Name,
	}

	for key, value := range envHeaders {
		req.Header.Set(key, value)
	}

	logger.Debug("Sending HTTP request")
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		logger.WithFields(log.Fields{
			"duration": duration,
			"error":    err.Error(),
		}).Error("HTTP request failed")
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	// Limit response body size to prevent memory issues
	limitedReader := io.LimitReader(resp.Body, MaxResponseBodySize)
	responseBody, readErr := io.ReadAll(limitedReader)
	if readErr != nil {
		logger.WithError(readErr).Warn("Failed to read response body")
		// Continue with empty body for status code checking
		responseBody = []byte{}
	}

	logger.WithFields(log.Fields{
		"status":        resp.StatusCode,
		"expected":      expectedStatus,
		"duration":      duration,
		"responseSize":  len(responseBody),
		"contentType":   resp.Header.Get("Content-Type"),
		"contentLength": resp.ContentLength,
	}).Debug("HTTP check completed")

	if resp.StatusCode != int(expectedStatus) {
		logger.WithFields(log.Fields{
			"actualStatus":   resp.StatusCode,
			"expectedStatus": expectedStatus,
			"responseBody":   string(responseBody),
		}).Error("HTTP check failed - status code mismatch")
		return fmt.Errorf("HTTP check failed: expected status %d, got %d. Response: %s",
			expectedStatus, resp.StatusCode, string(responseBody))
	}

	logger.Info("HTTP check completed successfully")
	return nil
}
