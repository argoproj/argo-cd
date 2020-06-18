package fixture

import (
	"bytes"
	"regexp"
	"strings"
	"text/template"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
)

// utility method to template a string using a map
func Tmpl(text string, values interface{}) string {
	parse, err := template.New(text).Parse(text)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	buf := new(bytes.Buffer)
	err = parse.Execute(buf, values)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	return buf.String()
}

// utility method to deal with white-space
func NormalizeOutput(text string) string {
	return regexp.MustCompile(` +`).
		ReplaceAllString(strings.TrimSpace(text), " ")
}
