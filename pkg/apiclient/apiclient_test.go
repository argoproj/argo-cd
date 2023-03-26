package apiclient

import (
	"net/http"
	"testing"
)

func TestParseHeaders_ValidHeaders(t *testing.T) {
	headerStrings := []string{"Content-Type:text/html", "X-Custom-Header:test"}
	expected := http.Header{
		"Content-Type":   []string{"text/html"},
		"X-Custom-Header": []string{"test"},
	}

	headers, err := parseHeaders(headerStrings)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !equalHeaders(headers, expected) {
		t.Errorf("Expected headers: %v, got: %v", expected, headers)
	}
}

func TestParseHeaders_InvalidHeaderFormat(t *testing.T) {
	headerStrings := []string{"Content-Type=text/html"}
	expectedError := "additional headers must be colon(:)-separated: Content-Type=text/html"

	headers, err := parseHeaders(headerStrings)

	if err == nil {
		t.Error("Expected error, got nil")
	} else if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}

	if headers != nil {
		t.Errorf("Expected headers to be nil, got: %v", headers)
	}
}

func TestParseHeaders_EmptyHeaderName(t *testing.T) {
	headerStrings := []string{":test"}
	expectedError := "additional headers must be colon(:)-separated: :test"

	headers, err := parseHeaders(headerStrings)

	if err == nil {
		t.Error("Expected error, got nil")
	} else if err.Error() != expectedError {
		t.Errorf("Expected error: %s, got: %s", expectedError, err.Error())
	}

	if headers != nil {
		t.Errorf("Expected headers to be nil, got: %v", headers)
	}
}

func equalHeaders(a, b http.Header) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		bv, ok := b[k]
		if !ok || len(av) != len(bv) {
			return false
		}
		for i, v := range av {
			if v != bv[i] {
				return false
			}
		}
	}

	return true
}
