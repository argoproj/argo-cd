package generators

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func (p *PhaseDeploymentProcessor) runHTTPCheck(ctx context.Context, appSet *argoprojiov1alpha1.ApplicationSet, check argoprojiov1alpha1.GeneratorPhaseCheck) error {
	if check.HTTP == nil {
		return fmt.Errorf("http check requires http field")
	}

	httpCheck := check.HTTP
	method := httpCheck.Method
	if method == "" {
		method = "GET"
	}

	expectedStatus := httpCheck.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: httpCheck.InsecureSkipVerify,
			},
		},
	}

	var body io.Reader
	if httpCheck.Body != "" {
		body = strings.NewReader(httpCheck.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, httpCheck.URL, body)
	if err != nil {
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

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	log.WithFields(log.Fields{
		"check":          check.Name,
		"url":            httpCheck.URL,
		"method":         method,
		"status":         resp.StatusCode,
		"expected":       expectedStatus,
		"duration":       duration,
		"response_size":  len(responseBody),
	}).Debug("HTTP check completed")

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("HTTP check failed: expected status %d, got %d. Response: %s", 
			expectedStatus, resp.StatusCode, string(responseBody))
	}

	return nil
}