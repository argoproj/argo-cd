package fixture

import (
	"bytes"
	"regexp"
	"strings"
	"text/template"

	. "github.com/argoproj/argo-cd/v2/util/errors"
)

// utility method to template a string using a map
func Tmpl(text string, values interface{}) string {
	parse, err := template.New(text).Parse(text)
	CheckError(err)
	buf := new(bytes.Buffer)
	err = parse.Execute(buf, values)
	CheckError(err)
	return buf.String()
}

// utility method to deal with white-space
func NormalizeOutput(text string) string {
	return regexp.MustCompile(` +`).
		ReplaceAllString(strings.TrimSpace(text), " ")
}
