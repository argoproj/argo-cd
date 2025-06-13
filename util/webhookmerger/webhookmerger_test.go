package webhookmerger

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
)

// MockWebhookHandler is a test double for WebhookHandler
type MockWebhookHandler struct {
	handleFunc func(w http.ResponseWriter, r *http.Request) error
}

// HandleRequest implements WebhookHandler for mocking
func (m *MockWebhookHandler) HandleRequest(w http.ResponseWriter, r *http.Request) error {
	return m.handleFunc(w, r)
}

func TestWebhookMerger_OK(t *testing.T) {
	acdHandler := &MockWebhookHandler{
		handleFunc: func(_ http.ResponseWriter, _ *http.Request) error {
			return nil
		},
	}
	appSetHandler := &MockWebhookHandler{
		handleFunc: func(_ http.ResponseWriter, _ *http.Request) error {
			return nil
		},
	}

	hook := test.NewGlobal()
	defer hook.Reset()

	merger := NewWebhookMerger(acdHandler, appSetHandler)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	w := httptest.NewRecorder()

	merger.Handler(w, req)

	if entry := hook.LastEntry(); entry != nil {
		t.Errorf("expected no logs, got: %s", hook.LastEntry().Message)
	}
}

func TestWebhookMerger_Error(t *testing.T) {
	tests := []struct {
		name               string
		acdError           error
		appSetError        error
		expectLogSubstring string
	}{
		{
			name:               "ACD",
			acdError:           errors.New("acd failure"),
			appSetError:        nil,
			expectLogSubstring: "error handling argo cd webhook: acd failure. maybe not suitable?",
		},
		{
			name:               "AppSet",
			acdError:           nil,
			appSetError:        errors.New("appset failure"),
			expectLogSubstring: "error handling application set webhook: appset failure. maybe not suitable?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()

			acdHandler := &MockWebhookHandler{
				handleFunc: func(_ http.ResponseWriter, _ *http.Request) error {
					return tt.acdError
				},
			}
			appSetHandler := &MockWebhookHandler{
				handleFunc: func(_ http.ResponseWriter, _ *http.Request) error {
					return tt.appSetError
				},
			}

			merger := NewWebhookMerger(acdHandler, appSetHandler)

			req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
			w := httptest.NewRecorder()

			merger.Handler(w, req)

			if !strings.Contains(hook.LastEntry().Message, tt.expectLogSubstring) {
				t.Errorf("expected log to contain %q, got: %s", tt.expectLogSubstring, hook.LastEntry().Message)
			}

			hook.Reset()
		})
	}
}
