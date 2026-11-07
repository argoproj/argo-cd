package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type azureRefreshRequest struct {
	method        string
	authorization string
	form          url.Values
}

func TestAzureRefreshTokenSource(t *testing.T) {
	requests := make(chan azureRefreshRequest, 3)
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		requests <- azureRefreshRequest{
			method:        r.Method,
			authorization: r.Header.Get("Authorization"),
			form:          r.PostForm,
		}

		currentRequest := requestCount.Add(1)
		response := map[string]any{
			"access_token": fmt.Sprintf("access-token-%d", currentRequest),
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		// Rotate the token after the first request. Subsequent responses omit
		// refresh_token to verify that the rotated token is preserved.
		if currentRequest == 1 {
			response["refresh_token"] = "refresh-token-2"
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	var assertionCount atomic.Int32
	source := &azureRefreshTokenSource{
		ctx: context.Background(),
		conf: &oauth2.Config{
			ClientID: "client-id",
			Endpoint: oauth2.Endpoint{
				TokenURL:  server.URL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		getClientAssertion: func(context.Context) (string, error) {
			currentAssertion := assertionCount.Add(1)
			return fmt.Sprintf("client-assertion-%d", currentAssertion), nil
		},
		refreshToken: "refresh-token-1",
	}

	firstToken, err := source.Token()
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", firstToken.AccessToken)
	assert.Equal(t, "refresh-token-2", firstToken.RefreshToken)

	secondToken, err := source.Token()
	require.NoError(t, err)
	assert.Equal(t, "access-token-2", secondToken.AccessToken)
	assert.Equal(t, "refresh-token-2", secondToken.RefreshToken)

	thirdToken, err := source.Token()
	require.NoError(t, err)
	assert.Equal(t, "access-token-3", thirdToken.AccessToken)
	assert.Equal(t, "refresh-token-2", thirdToken.RefreshToken)

	expectedRefreshTokens := []string{
		"refresh-token-1",
		"refresh-token-2",
		"refresh-token-2",
	}

	for i, expectedRefreshToken := range expectedRefreshTokens {
		request := <-requests

		assert.Equal(t, http.MethodPost, request.method)
		assert.Empty(t, request.authorization)
		assert.Equal(t, "client-id", request.form.Get("client_id"))
		assert.Equal(t, "refresh_token", request.form.Get("grant_type"))
		assert.Equal(t, expectedRefreshToken, request.form.Get("refresh_token"))
		assert.Equal(t, clientAssertionType, request.form.Get("client_assertion_type"))
		assert.Equal(t, fmt.Sprintf("client-assertion-%d", i+1), request.form.Get("client_assertion"))
		assert.Empty(t, request.form.Get("client_secret"))
	}

	assert.EqualValues(t, 3, assertionCount.Load())
}

func TestAzureRefreshTokenSourceRequiresRefreshToken(t *testing.T) {
	assertionCalled := false
	source := &azureRefreshTokenSource{
		ctx:  context.Background(),
		conf: &oauth2.Config{},
		getClientAssertion: func(context.Context) (string, error) {
			assertionCalled = true
			return "client-assertion", nil
		},
	}

	token, err := source.Token()

	require.ErrorIs(t, err, errAzureRefreshTokenMissing)
	assert.Nil(t, token)
	assert.False(t, assertionCalled)
}

func TestAzureRefreshTokenSourceReturnsClientAssertionError(t *testing.T) {
	expectedErr := errors.New("service account token unavailable")
	source := &azureRefreshTokenSource{
		ctx:          context.Background(),
		conf:         &oauth2.Config{},
		refreshToken: "refresh-token",
		getClientAssertion: func(context.Context) (string, error) {
			return "", expectedErr
		},
	}

	token, err := source.Token()

	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, token)
}
