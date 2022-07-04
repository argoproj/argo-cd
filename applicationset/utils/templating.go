package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	"github.com/valyala/fasttemplate"
	"sigs.k8s.io/yaml"
)

type Renderer interface {
	RenderTemplateParams(tmpl *argov1alpha1.Application, untypedTemplate *argoprojiov1alpha1.ApplicationSetUntypedTemplate, syncPolicy *argoprojiov1alpha1.ApplicationSetSyncPolicy, params map[string]string) (*argov1alpha1.Application, error)
}
type Render struct {
}

func (r *Render) RenderTemplateParams(tmpl *argov1alpha1.Application, untypedTemplate *argoprojiov1alpha1.ApplicationSetUntypedTemplate, syncPolicy *argoprojiov1alpha1.ApplicationSetSyncPolicy, params map[string]string) (*argov1alpha1.Application, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("application template is empty ")
	}
	if len(params) == 0 {
		return tmpl, nil
	}
	var replacedTmpl argov1alpha1.Application
	var replacedTmplStr string
	if untypedTemplate == nil {
		// interpolates the `${expression}` first and simple `{reference}` right after
		tmplBytes, err := json.Marshal(tmpl)
		if err != nil {
			return nil, err
		}
		replacedTmplStr, err = renderWithFastTemplateAndGoTemplate(string(tmplBytes), params)
		if err != nil {
			return nil, err
		}
		replacedTmplStr = renderWithFastTemplate(replacedTmplStr, params)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(replacedTmplStr), &replacedTmpl)
		if err != nil {
			return nil, err
		}
	} else {
		replacedTmplStr, err := renderWithGoTemplate(string(*untypedTemplate), params)
		if err != nil {
			return nil, err
		}
		// UnmarshalStrict to fail early and raise the fact that template
		// result produced not what is expected
		err = yaml.UnmarshalStrict([]byte(replacedTmplStr), &replacedTmpl)
		if err != nil {
			return nil, err
		}
	}
	// Add the 'resources-finalizer' finalizer if:
	// The template application doesn't have any finalizers, and:
	// a) there is no syncPolicy, or
	// b) there IS a syncPolicy, but preserveResourcesOnDeletion is set to false
	// See TestRenderTemplateParamsFinalizers in util_test.go for test-based definition of behaviour
	if (syncPolicy == nil || !syncPolicy.PreserveResourcesOnDeletion) &&
		(replacedTmpl.ObjectMeta.Finalizers == nil || len(replacedTmpl.ObjectMeta.Finalizers) == 0) {
		replacedTmpl.ObjectMeta.Finalizers = []string{"resources-finalizer.argocd.argoproj.io"}
	}
	return &replacedTmpl, nil
}

// renderWithGoTemplate executes gotemplate with sprig functions against the raw (untyped) template
func renderWithGoTemplate(rawTemplate string, data map[string]string) (string, error) {
	goTemplate, err := template.New("").Option("missingkey=zero").Funcs(createFuncMap()).Parse(rawTemplate)
	if err != nil {
		return "", err
	}
	var tplString bytes.Buffer
	err = goTemplate.Execute(&tplString, data)
	if err != nil {
		return "", err
	}
	return tplString.String(), nil
}

// renderWithFastTemplateAndGoTemplate executes string substitution with the result of gotemplate run
// for every token found
func renderWithFastTemplateAndGoTemplate(rawTemplate string, data map[string]string) (string, error) {
	fstTmpl := fasttemplate.New(rawTemplate, "${{", "}}")
	replacedTmplStr, err := fstTmpl.ExecuteFuncStringWithErr(func(w io.Writer, tag string) (int, error) {
		trimmedTag := strings.TrimSpace(tag)
		// json control characters here are double escaped, what was a double quote " becomes \" when
		// unmarshalled to ApplicationSet and then \\\" when marshaled to json
		// this becomes a problem with gotemplate trying to parse the token, so unquote the string
		unquotedTag, err := strconv.Unquote(`"` + trimmedTag + `"`)
		if err != nil {
			return 0, err
		}
		// wrapping back in {{}} for gotemplate to identify the expression
		gotemplateTag := fmt.Sprintf("{{%s}}", unquotedTag)
		goTemplate, err := template.New("").Option("missingkey=zero").Funcs(createFuncMap()).Parse(gotemplateTag)
		if err != nil {
			return 0, err
		}
		var tplString bytes.Buffer
		err = goTemplate.Execute(&tplString, data)
		if err != nil {
			return 0, err
		}
		// The following escapes any special characters (e.g. newlines, tabs, etc...)
		// in preparation for substitution
		replacement := strconv.Quote(tplString.String())
		replacement = replacement[1 : len(replacement)-1]
		return w.Write([]byte(replacement))
	})
	if err != nil {
		return "", err
	}
	return replacedTmplStr, nil
}

// replaceWithFastTemplate executes basic string substitution of a template with replacement values.
func renderWithFastTemplate(rawTemplate string, data map[string]string) string {
	fstTmpl := fasttemplate.New(rawTemplate, "{{", "}}")
	replacedTmplStr := fstTmpl.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		trimmedTag := strings.TrimSpace(tag)
		replacement, ok := data[trimmedTag]
		if len(trimmedTag) == 0 || !ok {
			return w.Write([]byte(fmt.Sprintf("{{%s}}", tag)))
		}
		// The following escapes any special characters (e.g. newlines, tabs, etc...)
		// in preparation for substitution
		replacement = strconv.Quote(replacement)
		replacement = replacement[1 : len(replacement)-1]
		return w.Write([]byte(replacement))
	})
	return replacedTmplStr
}
func ToYaml(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func createFuncMap() template.FuncMap {
	funcMap := sprig.TxtFuncMap()
	extraFuncMap := template.FuncMap{
		"toYaml": ToYaml,
	}
	for name, f := range extraFuncMap {
		funcMap[name] = f
	}
	return funcMap
}
