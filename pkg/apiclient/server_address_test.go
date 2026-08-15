package apiclient

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClientRejectsHTTPURLServerAddress(t *testing.T) {
	_, err := NewClient(&ClientOptions{
		ConfigPath: filepath.Join(t.TempDir(), "config"),
		ServerAddr: "https://localhost:8080",
	})

	require.EqualError(t, err, `server address "https://localhost:8080" must not include a URL scheme; use "localhost:8080" instead`)
}

func TestNewClientRejectsHTTPURLServerAddressFromEnvironment(t *testing.T) {
	t.Setenv(EnvArgoCDServer, "https://localhost:8080")

	_, err := NewClient(&ClientOptions{
		ConfigPath: filepath.Join(t.TempDir(), "config"),
	})

	require.EqualError(t, err, `server address "https://localhost:8080" must not include a URL scheme; use "localhost:8080" instead`)
}

func TestNewClientServerAddressOptionOverridesEnvironment(t *testing.T) {
	t.Setenv(EnvArgoCDServer, "https://localhost:8080")

	client, err := NewClient(&ClientOptions{
		ConfigPath: filepath.Join(t.TempDir(), "config"),
		ServerAddr: "localhost:8080",
		GRPCWeb:    true,
	})

	require.NoError(t, err)
	require.Equal(t, "localhost:8080", client.ClientOptions().ServerAddr)
}
