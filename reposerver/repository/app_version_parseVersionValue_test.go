package repository

import (
	"bytes"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

// Correctly extracts version when jsonPathExpression matches a string value
func TestParseVersionValueWithString(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": "1.0.0",
	}
	jsonPathExpression := "$.version"
	expectedVersion := "1.0.0"

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles non-string version values gracefully
func TestParseVersionValueWithNonString(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": 123,
	}
	jsonPathExpression := "$.version"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Logs appropriate message when version value is non-string
func TestParseVersionValueWithNonString1(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": 1.0,
	}
	jsonPathExpression := "$.version"

	// Capture log output
	var logOutput bytes.Buffer
	log.SetOutput(&logOutput)

	_ = parseVersionValue(jsonPathExpression, jsonObj)

	expectedLogMessage := "Version value is not a string. Got: 1"
	if !strings.Contains(logOutput.String(), expectedLogMessage) {
		t.Errorf("Expected log message containing '%s', but got '%s'", expectedLogMessage, logOutput.String())
	}
}

// Correctly extracts version when jsonPathExpression matches an array with a string value
func TestParseVersionValueWithArray(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": []interface{}{"1.0.0"},
	}
	jsonPathExpression := "$.version"
	expectedVersion := "1.0.0"

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Returns empty string when jsonPathExpression does not match any value
func TestParseVersionValueWhenJsonPathExpressionDoesNotMatch(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": "1.0.0",
	}
	jsonPathExpression := "$.nonexistent"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles nil jsonObj without crashing
func TestParseVersionValueWithNilJSONObj(t *testing.T) {
	var jsonObj interface{}
	jsonPathExpression := "$.version"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles empty jsonPathExpression without crashing
func TestParseVersionValueWithEmptyExpression(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": "1.0.0",
	}
	jsonPathExpression := ""
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles jsonPathExpression that matches an empty array
func TestParseVersionValueWithEmptyArray(t *testing.T) {
	jsonObj := []interface{}{}
	jsonPathExpression := "$.version"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles jsonPathExpression that matches a non-string array
func TestParseVersionValueWithNonStringArray(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": []interface{}{1.0, 2.0, 3.0},
	}
	jsonPathExpression := "$.version"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Handles jsonPathExpression that matches a nil value
func TestParseVersionValueWithNil(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": nil,
	}
	jsonPathExpression := "$.version"
	expectedVersion := ""

	actualVersion := parseVersionValue(jsonPathExpression, jsonObj)

	if actualVersion != expectedVersion {
		t.Errorf("Expected version %s, but got %s", expectedVersion, actualVersion)
	}
}

// Logs appropriate message when version value is nil
func TestParseVersionValueWithNilVersion(t *testing.T) {
	jsonObj := map[string]interface{}{
		"version": nil,
	}
	jsonPathExpression := "$.version"

	var buf bytes.Buffer
	log.SetOutput(&buf)

	_ = parseVersionValue(jsonPathExpression, jsonObj)

	logOutput := buf.String()
	expectedLog := "Version value is not a string. Got: nil"
	if !strings.Contains(logOutput, expectedLog) {
		t.Errorf("Expected log message: %s, but got: %s", expectedLog, logOutput)
	}
}
