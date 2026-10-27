package initialize

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StringFlag struct {
	// The exact value provided on the flag
	value string
}

func (f StringFlag) String() string {
	return f.value
}

func (f *StringFlag) Set(value string) error {
	f.value = value
	return nil
}

func (f *StringFlag) Type() string {
	return "string"
}

func Test_FlagContextNotChanged(t *testing.T) {
	res := RetrieveContextIfChanged(&flag.Flag{
		Name:                "",
		Shorthand:           "",
		Usage:               "",
		Value:               &StringFlag{value: "test"},
		DefValue:            "",
		Changed:             false,
		NoOptDefVal:         "",
		Deprecated:          "",
		Hidden:              false,
		ShorthandDeprecated: "",
		Annotations:         nil,
	})

	assert.Empty(t, res)
}

func Test_FlagContextChanged(t *testing.T) {
	res := RetrieveContextIfChanged(&flag.Flag{
		Name:                "",
		Shorthand:           "",
		Usage:               "",
		Value:               &StringFlag{value: "test"},
		DefValue:            "",
		Changed:             true,
		NoOptDefVal:         "",
		Deprecated:          "",
		Hidden:              false,
		ShorthandDeprecated: "",
		Annotations:         nil,
	})

	assert.Equal(t, "test", res)
}

func Test_FlagContextNil(t *testing.T) {
	res := RetrieveContextIfChanged(&flag.Flag{
		Name:                "",
		Shorthand:           "",
		Usage:               "",
		Value:               nil,
		DefValue:            "",
		Changed:             false,
		NoOptDefVal:         "",
		Deprecated:          "",
		Hidden:              false,
		ShorthandDeprecated: "",
		Annotations:         nil,
	})

	assert.Empty(t, res)
}

func Test_InitCommand_FiltersUnsupportedKubectlFlags(t *testing.T) {
	cmd := &cobra.Command{}

	InitCommand(cmd)

	unsupported := []string{
		"disable-compression",
		"certificate-authority",
		"client-certificate",
		"client-key",
		"as",
		"as-group",
		"as-uid",
		"tls-server-name",
	}

	for _, f := range unsupported {
		assert.Nil(t, cmd.Flags().Lookup(f))
	}

	assert.NotNil(t, cmd.Flags().Lookup("context"))
	assert.NotNil(t, cmd.Flags().Lookup("namespace"))
}

func TestInitCommandKubeConfigInheritedBySubcommands(t *testing.T) {
	for _, args := range [][]string{
		{"list"},
		{"list", "--kubeconfig", "custom.yaml"},
		{"--kubeconfig", "custom.yaml", "list"},
	} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			cmd := InitCommand(&cobra.Command{Use: "app"})
			called := false
			cmd.AddCommand(&cobra.Command{
				Use: "list",
				Run: func(child *cobra.Command, _ []string) {
					called = true
					kubeconfig := child.Flag("kubeconfig")
					require.NotNil(t, kubeconfig)
					assert.Equal(t, len(args) > 1, kubeconfig.Changed)
					if kubeconfig.Changed {
						assert.Equal(t, "custom.yaml", kubeconfig.Value.String())
					}
				},
			})
			cmd.SetArgs(args)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			assert.True(t, called)
		})
	}
}
