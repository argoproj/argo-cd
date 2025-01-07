package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Hello, World!"))
		if err != nil {
			assert.NoError(t, fmt.Errorf("Error Write %w", err))
		}
	}))
	defer server.Close()

	var clientOptionFns []ClientOptionFunc
	_, err := NewClient(server.URL, clientOptionFns...)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
}

func TestClientDo(t *testing.T) {
	ctx := context.Background()

	for _, c := range []struct {
		name            string
		params          map[string]string
		content         []byte
		fakeServer      *httptest.Server
		clientOptionFns []ClientOptionFunc
		expected        []map[string]interface{}
		expectedCode    int
		expectedError   error
	}{
		{
			name: "Simple",
			params: map[string]string{
				"pkey1": "val1",
				"pkey2": "val2",
			},
			fakeServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`[{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
				 }]`))
				if err != nil {
					assert.NoError(t, fmt.Errorf("Error Write %w", err))
				}
			})),
			clientOptionFns: nil,
			expected: []map[string]interface{}{
				{
					"key1": "val1",
					"key2": map[string]interface{}{
						"key2_1": "val2_1",
						"key2_2": map[string]interface{}{
							"key2_2_1": "val2_2_1",
						},
					},
					"key3": float64(123),
				},
			},
			expectedCode:  200,
			expectedError: nil,
		},
		{
			name: "With Token",
			params: map[string]string{
				"pkey1": "val1",
				"pkey2": "val2",
			},
			fakeServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer "+string("test-token") {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`[{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
				 }]`))
				if err != nil {
					assert.NoError(t, fmt.Errorf("Error Write %w", err))
				}
			})),
			clientOptionFns: nil,
			expected:        []map[string]interface{}(nil),
			expectedCode:    401,
			expectedError:   fmt.Errorf("API error with status code 401: "),
		},
	} {
		cc := c
		t.Run(cc.name, func(t *testing.T) {
			defer cc.fakeServer.Close()

			client, err := NewClient(cc.fakeServer.URL, cc.clientOptionFns...)
			if err != nil {
				t.Fatalf("NewClient returned unexpected error: %v", err)
			}

			req, err := client.NewRequest("POST", "", cc.params, nil)
			if err != nil {
				t.Fatalf("NewRequest returned unexpected error: %v", err)
			}

			var data []map[string]interface{}

			resp, err := client.Do(ctx, req, &data)

			if cc.expectedError != nil {
				assert.EqualError(t, err, cc.expectedError.Error())
			} else {
				assert.Equal(t, cc.expectedCode, resp.StatusCode)
				assert.Equal(t, cc.expected, data)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckResponse(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(`{"error":"invalid_request","description":"Invalid token"}`)),
	}

	err := CheckResponse(resp)
	if err == nil {
		t.Error("Expected an error, got nil")
	}

	expected := "API error with status code 400: invalid_request"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}
