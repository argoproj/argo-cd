package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasttemplate"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

type Renderer interface {
	RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsetv1.ApplicationSetSyncPolicy, params map[string]string) (*argoappsv1.Application, error)
}

type Render struct {
}

func (r *Render) RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsetv1.ApplicationSetSyncPolicy, params map[string]string) (*argoappsv1.Application, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("application template is empty ")
	}

	if len(params) == 0 {
		return tmpl, nil
	}

	tmplBytes, err := json.Marshal(tmpl)
	if err != nil {
		return nil, err
	}

	fstTmpl := fasttemplate.New(string(tmplBytes), "{{", "}}")
	replacedTmplStr, err := r.replace(fstTmpl, params, true)
	if err != nil {
		return nil, err
	}

	var replacedTmpl argoappsv1.Application
	err = json.Unmarshal([]byte(replacedTmplStr), &replacedTmpl)
	if err != nil {
		return nil, err
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

// Replace executes basic string substitution of a template with replacement values.
// 'allowUnresolved' indicates whether or not it is acceptable to have unresolved variables
// remaining in the substituted template.
func (r *Render) replace(fstTmpl *fasttemplate.Template, replaceMap map[string]string, allowUnresolved bool) (string, error) {
	var unresolvedErr error
	replacedTmpl := fstTmpl.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {

		trimmedTag := strings.TrimSpace(tag)

		replacement, ok := replaceMap[trimmedTag]
		if len(trimmedTag) == 0 || !ok {
			if allowUnresolved {
				// just write the same string back
				return w.Write([]byte(fmt.Sprintf("{{%s}}", tag)))
			}
			unresolvedErr = fmt.Errorf("failed to resolve {{%s}}", tag)
			return 0, nil
		}
		// The following escapes any special characters (e.g. newlines, tabs, etc...)
		// in preparation for substitution
		replacement = strconv.Quote(replacement)
		replacement = replacement[1 : len(replacement)-1]
		return w.Write([]byte(replacement))
	})
	if unresolvedErr != nil {
		return "", unresolvedErr
	}

	return replacedTmpl, nil
}

// Log a warning if there are unrecognized generators
func CheckInvalidGenerators(applicationSetInfo *argoappsetv1.ApplicationSet) {
	hasInvalidGenerators, invalidGenerators := invalidGenerators(applicationSetInfo)
	if len(invalidGenerators) > 0 {
		gnames := []string{}
		for n := range invalidGenerators {
			gnames = append(gnames, n)
		}
		sort.Strings(gnames)
		aname := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators: %s"
		log.Warnf(msg, aname, strings.Join(gnames, ", "))
	} else if hasInvalidGenerators {
		name := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators"
		log.Warnf(msg, name)
	}
}

// Return true if there are unknown generators specified in the application set.  If we can discover the names
// of these generators, return the names as the keys in a map
func invalidGenerators(applicationSetInfo *argoappsetv1.ApplicationSet) (bool, map[string]bool) {
	names := make(map[string]bool)
	hasInvalidGenerators := false
	for index, generator := range applicationSetInfo.Spec.Generators {
		v := reflect.Indirect(reflect.ValueOf(generator))
		found := false
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanInterface() {
				continue
			}
			if !reflect.ValueOf(field.Interface()).IsNil() {
				found = true
				break
			}
		}
		if !found {
			hasInvalidGenerators = true
			addInvalidGeneratorNames(names, applicationSetInfo, index)
		}
	}
	return hasInvalidGenerators, names
}

func addInvalidGeneratorNames(names map[string]bool, applicationSetInfo *argoappsetv1.ApplicationSet, index int) {
	// The generator names are stored in the "kubectl.kubernetes.io/last-applied-configuration" annotation
	config := applicationSetInfo.ObjectMeta.Annotations["kubectl.kubernetes.io/last-applied-configuration"]
	var values map[string]interface{}
	err := json.Unmarshal([]byte(config), &values)
	if err != nil {
		log.Warnf("couldn't unmarshal kubectl.kubernetes.io/last-applied-configuration: %+v", config)
		return
	}

	spec, ok := values["spec"].(map[string]interface{})
	if !ok {
		log.Warn("coundn't get spec from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	generators, ok := spec["generators"].([]interface{})
	if !ok {
		log.Warn("coundn't get generators from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	if index >= len(generators) {
		log.Warnf("index %d out of range %d for generator in kubectl.kubernetes.io/last-applied-configuration", index, len(generators))
		return
	}

	generator, ok := generators[index].(map[string]interface{})
	if !ok {
		log.Warn("coundn't get generator from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	for key := range generator {
		names[key] = true
		break
	}
}

func NormalizeBitbucketBasePath(basePath string) string {
	if strings.HasSuffix(basePath, "/rest/") {
		return strings.TrimSuffix(basePath, "/")
	}
	if !strings.HasSuffix(basePath, "/rest") {
		return basePath + "/rest"
	}
	return basePath
}
