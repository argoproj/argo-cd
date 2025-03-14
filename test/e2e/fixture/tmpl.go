package fixture

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

// utility method to template a string using a map
func Tmpl(t *testing.T, text string, values any) string {
	t.Helper()
	parse, err := template.New(text).Parse(text)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	err = parse.Execute(buf, values)
	require.NoError(t, err)
	return buf.String()
}

// utility method to deal with white-space
func NormalizeOutput(text string) string {
	return regexp.MustCompile(` +`).
		ReplaceAllString(strings.TrimSpace(text), " ")
}
