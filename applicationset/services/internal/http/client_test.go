package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "Failed to create client")
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
			expectedCode:  http.StatusOK,
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
			expectedCode:    http.StatusUnauthorized,
			expectedError:   errors.New("API error with status code 401: "),
		},
	} {
		cc := c
		t.Run(cc.name, func(t *testing.T) {
			defer cc.fakeServer.Close()

			client, err := NewClient(cc.fakeServer.URL, cc.clientOptionFns...)
			require.NoError(t, err, "NewClient returned unexpected error")

			req, err := client.NewRequest("POST", "", cc.params, nil)
			require.NoError(t, err, "NewRequest returned unexpected error")

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
	require.EqualError(t, err, "API error with status code 400: invalid_request")
}
