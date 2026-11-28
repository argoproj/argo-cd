package commands

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
)

func TestNewUpgradeCmd(t *testing.T) {
	tests := []struct {
		name          string
		clientVersion string
		serverVersion string
		skipChecks    bool
		upgradeTag    string
		expected      string
	}{
		{
			name:          "without upgrade tag",
			clientVersion: "v3.1.0",
			serverVersion: "v2.14.0",
			skipChecks:    true,
			upgradeTag:    "",
			expected:      fmt.Sprintf("%s%s to %s", "Performing checks for upgrade from ", "2.14.0", "3.1.0"),
		},
		{
			name:          "with upgrade tag",
			clientVersion: "v3.1.0",
			serverVersion: "v2.14.0",
			skipChecks:    true,
			upgradeTag:    "v3.0.0",
			expected:      fmt.Sprintf("%s%s to %s", "Performing checks for upgrade from ", "2.14.0", "3.0.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := NewUpgradeCmd(&argocdclient.ClientOptions{}, tt.clientVersion, tt.serverVersion, tt.skipChecks)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{cliName, "upgrade", "--upgrade-tag", tt.upgradeTag})
			require.NoError(t, cmd.Execute(), "error")
			require.Contains(t, buf.String(), tt.expected)
		})
	}
}
