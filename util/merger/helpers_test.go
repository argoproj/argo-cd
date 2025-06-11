package merger

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyRequest_Ok(t *testing.T) {
	for _, tt := range []struct {
		name         string
		method       string
		header       http.Header
		originalBody io.ReadCloser
		expectedBody string
	}{
		{
			name:   "NilBodyWithPostAndNoHeaders",
			method: http.MethodPost,
		},
		{
			name:         "ValidBodyWithPostAndHeader",
			method:       http.MethodPost,
			header:       http.Header{"X-Test": []string{"abc"}},
			originalBody: io.NopCloser(strings.NewReader("hello world")),
			expectedBody: "hello world",
		},
		{
			name:   "ValidBodyWithPutAndMultipleHeaders",
			method: http.MethodPut,
			header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Custom":     []string{"value"},
			},
			originalBody: io.NopCloser(strings.NewReader("some data")),
			expectedBody: "some data",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := &http.Request{
				Method: tt.method,
				Header: tt.header.Clone(),
				Body:   tt.originalBody,
			}

			copied, err := copyRequest(original)
			require.NoError(t, err)
			require.NotSame(t, copied, original)

			assert.Equal(t, tt.method, copied.Method)
			assert.Equal(t, tt.header, copied.Header)

			if tt.expectedBody != "" {
				copiedBody, err := io.ReadAll(copied.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBody, string(copiedBody), "Copied body mismatch")

				originalBody, err := io.ReadAll(original.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBody, string(originalBody), "Original body mismatch")
			}
		})
	}
}

// Test copyRequest with a body that returns error on Read
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("request body read error")
}

func (errReader) Close() error {
	return nil
}

func TestCopyRequest_Error(t *testing.T) {
	for _, tt := range []struct {
		name         string
		originalBody io.ReadCloser
	}{
		{
			name:         "ErrorReadingBody",
			originalBody: errReader{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := &http.Request{
				Body: tt.originalBody,
			}

			_, err := copyRequest(original)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "request body read error")
		})
	}
}
