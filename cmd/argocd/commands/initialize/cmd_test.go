package initialize

import (
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/cli"
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

func Test_InitCommand_ExposesExactlySupportedKubectlFlags(t *testing.T) {
	cmd := &cobra.Command{}

	InitCommand(cmd)

	var exposed []string
	cmd.Flags().VisitAll(func(f *flag.Flag) {
		exposed = append(exposed, f.Name)
	})

	// The exposed set must be exactly the allowlist: this fails if a kubectl
	// flag not supported by the argocd CLI (e.g. a flag newly added upstream
	// in client-go) ever leaks into the command help again.
	assert.ElementsMatch(t, supportedKubectlFlags, exposed)
}

func Test_InitCommand_UnsupportedKubectlFlagsNotExposed(t *testing.T) {
	cmd := &cobra.Command{}

	InitCommand(cmd)

	// kubectl REST flags that historically leaked into the argocd CLI and had
	// to be removed one by one (#25977, #27711, #18198), plus "server", which
	// clashes with argocd's own --server flag.
	unsupported := []string{
		"disable-compression",
		"certificate-authority",
		"client-certificate",
		"client-key",
		"as",
		"as-group",
		"as-uid",
		"tls-server-name",
		"request-timeout",
		"server",
	}

	for _, f := range unsupported {
		assert.Nil(t, cmd.Flags().Lookup(f), "--%s must not be exposed on argocd commands", f)
	}
}

func Test_InitCommand_SupportedFlagsExistUpstream(t *testing.T) {
	// Guard against client-go renaming or dropping a flag the allowlist
	// relies on: every supported flag must still be registered by
	// cli.AddKubectlFlagsToSet.
	flags := flag.NewFlagSet("tmp", flag.ContinueOnError)
	cli.AddKubectlFlagsToSet(flags)

	for _, name := range supportedKubectlFlags {
		assert.NotNil(t, flags.Lookup(name), "--%s is missing from the client-go kubectl flag set", name)
	}
}

func Test_InitCommand_SupportedFlagsParse(t *testing.T) {
	cmd := &cobra.Command{}

	InitCommand(cmd)

	err := cmd.Flags().Parse([]string{"--context", "test-ctx", "-n", "argocd", "--kubeconfig", "/tmp/config"})
	require.NoError(t, err)
	assert.Equal(t, "test-ctx", RetrieveContextIfChanged(cmd.Flag("context")))
}
