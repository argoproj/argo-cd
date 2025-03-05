package fixture

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

// utility method to template a string using a map
func Tmpl(t *testing.T, text string, values any) string {
	parse, err := template.New(text).Parse(text)
	errors.NewHandler(t).CheckForErr(err)
	buf := new(bytes.Buffer)
	err = parse.Execute(buf, values)
	errors.NewHandler(t).CheckForErr(err)
	return buf.String()
}

// utility method to deal with white-space
func NormalizeOutput(text string) string {
	return regexp.MustCompile(` +`).
		ReplaceAllString(strings.TrimSpace(text), " ")
}
