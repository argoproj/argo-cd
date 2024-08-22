package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
)

func Test_IsDefined(t *testing.T) {
	testCases := []struct {
		name     string
		discover Discover
		expected bool
	}{
		{
			name:     "empty discover",
			discover: Discover{},
			expected: false,
		},
		{
			name: "discover with find",
			discover: Discover{
				Find: Find{
					Glob: "glob",
				},
			},
			expected: true,
		},
		{
			name: "discover with fileName",
			discover: Discover{
				FileName: "fileName",
			},
			expected: true,
		},
		{
			name: "discover with empty command",
			discover: Discover{
				Find: Find{
					Command: Command{
						Command: []string{},
					},
				},
			},
			expected: false,
		},
		{
			name: "discover with command",
			discover: Discover{
				Find: Find{
					Command: Command{
						Command: []string{"command"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()

			actual := tcc.discover.IsDefined()
			assert.Equal(t, tcc.expected, actual)
		})
	}
}

func Test_ReadPluginConfig(t *testing.T) {
	testCases := []struct {
		name         string
		fileContents string
		expected     *PluginConfig
		expectedErr  string
	}{
		{
			name: "empty metadata",
			fileContents: `
metadata:
`,
			expected:    nil,
			expectedErr: "invalid plugin configuration file. metadata.name should be non-empty.",
		},
		{
			name: "empty metadata name",
			fileContents: `
metadata:
  name: ""
`,
			expected:    nil,
			expectedErr: "invalid plugin configuration file. metadata.name should be non-empty.",
		},
		{
			name: "invalid kind",
			fileContents: `
kind: invalid
metadata:	
  name: name
`,
			expected:    nil,
			expectedErr: "invalid plugin configuration file. kind should be ConfigManagementPlugin, found invalid",
		},
		{
			name: "empty generate command",
			fileContents: `
kind: ConfigManagementPlugin
metadata:
  name: name
`,
			expected:    nil,
			expectedErr: "invalid plugin configuration file. spec.generate command should be non-empty",
		},
		{
			name: "valid config",
			fileContents: `
kind: ConfigManagementPlugin
metadata:
  name: name
spec:
  generate:
    command: [command]
`,
			expected: &PluginConfig{
				TypeMeta: v1.TypeMeta{
					Kind: ConfigManagementPluginKind,
				},
				Metadata: v1.ObjectMeta{
					Name: "name",
				},
				Spec: PluginConfigSpec{
					Generate: Command{
						Command: []string{"command"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			// write test string to temporary file
			tempDir := t.TempDir()
			tempFile, err := os.Create(filepath.Join(tempDir, "plugin.yaml"))
			require.NoError(t, err)
			err = tempFile.Close()
			require.NoError(t, err)
			err = os.WriteFile(tempFile.Name(), []byte(tcc.fileContents), 0o644)
			require.NoError(t, err)
			config, err := ReadPluginConfig(tempDir)
			if tcc.expectedErr != "" {
				require.EqualError(t, err, tcc.expectedErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tcc.expected, config)
		})
	}
}

func Test_PluginConfig_Address(t *testing.T) {
	testCases := []struct {
		name     string
		config   *PluginConfig
		expected string
	}{
		{
			name: "no version specified",
			config: &PluginConfig{
				TypeMeta: v1.TypeMeta{
					Kind: ConfigManagementPluginKind,
				},
				Metadata: v1.ObjectMeta{
					Name: "name",
				},
			},
			expected: "name",
		},
		{
			name: "version specified",
			config: &PluginConfig{
				TypeMeta: v1.TypeMeta{
					Kind: ConfigManagementPluginKind,
				},
				Metadata: v1.ObjectMeta{
					Name: "name",
				},
				Spec: PluginConfigSpec{
					Version: "version",
				},
			},
			expected: "name-version",
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			actual := tcc.config.Address()
			expectedAddress := fmt.Sprintf("%s/%s.sock", common.GetPluginSockFilePath(), tcc.expected)
			assert.Equal(t, expectedAddress, actual)
		})
	}
}
