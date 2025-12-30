package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommand(t *testing.T) {
	t.Parallel()

	cmd := NewCommand()

	t.Run("has correct name and description", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "argocd-k8s-auth", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has logging flags", func(t *testing.T) {
		t.Parallel()
		logformatFlag := cmd.PersistentFlags().Lookup("logformat")
		assert.NotNil(t, logformatFlag)
		assert.Equal(t, "text", logformatFlag.DefValue)

		loglevelFlag := cmd.PersistentFlags().Lookup("loglevel")
		assert.NotNil(t, loglevelFlag)
		assert.Equal(t, "info", loglevelFlag.DefValue)
	})

	t.Run("has expected subcommands", func(t *testing.T) {
		t.Parallel()
		subcommands := cmd.Commands()

		var subcommandNames []string
		for _, c := range subcommands {
			subcommandNames = append(subcommandNames, c.Use)
		}

		assert.Contains(t, subcommandNames, "aws")
		assert.Contains(t, subcommandNames, "gcp")
		assert.Contains(t, subcommandNames, "azure")
		assert.Contains(t, subcommandNames, "version")
	})
}
