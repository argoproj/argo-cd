package initialize

import (
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
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
	}

	for _, f := range unsupported {
		assert.Nil(t, cmd.Flags().Lookup(f))
	}

	assert.NotNil(t, cmd.Flags().Lookup("context"))
	assert.NotNil(t, cmd.Flags().Lookup("namespace"))
}
