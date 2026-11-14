package oci

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentialScopeIsolatesViews(t *testing.T) {
	first := credsScope(Creds{Username: "reader", Password: "one"})
	require.Equal(t, first, credsScope(Creds{Username: "reader", Password: "one"}))
	require.NotEqual(t, first, credsScope(Creds{Username: "reader", Password: "two"}))
	require.Len(t, strings.TrimPrefix(first, "argon2id-v1:"), 64)
	require.NotEqual(t, credsScope(Creds{Username: "a\x00b", Password: "c"}), credsScope(Creds{Username: "a", Password: "b\x00c"}))
	require.NotEqual(t, first, credsScope(Creds{Username: "reader", Password: "one", InsecureHTTPOnly: true}))
}
