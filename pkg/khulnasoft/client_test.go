package khulnasoft

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
)

type MockRoundTripper struct {
	mock.Mock
}

func (r *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := r.Called(req)

	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*http.Response), args.Error(1)
}

func TestKhulnasoftClient_SendEvent(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		authToken string
		payload   []byte
		wantErr   string
		beforeFn  func(t *testing.T, rt *MockRoundTripper)
	}{
		{
			name:      "should return nil when all is good",
			baseURL:   "http://some.host",
			authToken: "some-token",
			payload:   []byte(`{"key": "value"}`),
			beforeFn: func(t *testing.T, rt *MockRoundTripper) {
				rt.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "POST", req.Method, "invalid request method")
					assert.Equal(t, "http://some.host/2.0/api/events", req.URL.String(), "invalid request URL")
					assert.Equal(t, "some-token", req.Header.Get("Authorization"), "missing or invalid Authorization header")
					assert.Equal(t, "application/json", req.Header.Get("Content-Type"), "missing or invalid Content-Type header")
					reader, err := gzip.NewReader(req.Body)
					require.NoError(t, err, "failed to create gzip reader")
					defer reader.Close()
					body, err := io.ReadAll(reader)
					require.NoError(t, err, "failed to read request body")
					assert.JSONEq(t, `{"data":{"key":"value"}}`, string(body), "invalid request body")
				}).Return(&http.Response{
					StatusCode: 200,
				}, nil)
			},
		},
		{
			name:      "should create correct url when baseUrl ends with '/'",
			baseURL:   "http://some.host/",
			authToken: "some-token",
			payload:   []byte(`{"key": "value"}`),
			beforeFn: func(t *testing.T, rt *MockRoundTripper) {
				rt.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "http://some.host/2.0/api/events", req.URL.String(), "invalid request URL")
				}).Return(&http.Response{
					StatusCode: 200,
				}, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRT := &MockRoundTripper{}
			c := &KhulnasoftClient{
				cfConfig: &KhulnasoftConfig{
					BaseURL:   tt.baseURL,
					AuthToken: tt.authToken,
				},
				httpClient: &http.Client{
					Transport: mockRT,
				},
			}
			event := &events.Event{
				Payload: tt.payload,
			}
			tt.beforeFn(t, mockRT)
			if err := c.SendEvent(context.Background(), "appName", event); err != nil || tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
