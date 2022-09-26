package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unsafe"

	"github.com/Masterminds/sprig"
	"github.com/valyala/fasttemplate"

	log "github.com/sirupsen/logrus"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
	sprigFuncMap["normalize"] = SanitizeName
}

type Renderer interface {
	RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsv1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool) (*argoappsv1.Application, error)
}

type Render struct {
}

func copyValueIntoUnexported(destination, value reflect.Value) {
	reflect.NewAt(destination.Type(), unsafe.Pointer(destination.UnsafeAddr())).
		Elem().
		Set(value)
}

func copyUnexported(copy, original reflect.Value) {
	var unexported = reflect.NewAt(original.Type(), unsafe.Pointer(original.UnsafeAddr())).Elem()
	copyValueIntoUnexported(copy, unexported)
}

// This function is in charge of searching all String fields of the object recursively and apply templating
// thanks to https://gist.github.com/randallmlough/1fd78ec8a1034916ca52281e3b886dc7
func (r *Render) deeplyReplace(copy, original reflect.Value, replaceMap map[string]interface{}, useGoTemplate bool) error {
	switch original.Kind() {
	// The first cases handle nested structures and translate them recursively
	// If it is a pointer we need to unwrap and call once again
	case reflect.Ptr:
		// To get the actual value of the original we have to call Elem()
		// At the same time this unwraps the pointer so we don't end up in
		// an infinite recursion
		originalValue := original.Elem()
		// Check if the pointer is nil
		if !originalValue.IsValid() {
			return nil
		}
		// Allocate a new object and set the pointer to it
		if originalValue.CanSet() {
			copy.Set(reflect.New(originalValue.Type()))
		} else {
			copyUnexported(copy, original)
		}
		// Unwrap the newly created pointer
		if err := r.deeplyReplace(copy.Elem(), originalValue, replaceMap, useGoTemplate); err != nil {
			return err
		}

	// If it is an interface (which is very similar to a pointer), do basically the
	// same as for the pointer. Though a pointer is not the same as an interface so
	// note that we have to call Elem() after creating a new object because otherwise
	// we would end up with an actual pointer
	case reflect.Interface:
		// Get rid of the wrapping interface
		originalValue := original.Elem()
		// Create a new object. Now new gives us a pointer, but we want the value it
		// points to, so we have to call Elem() to unwrap it
		copyValue := reflect.New(originalValue.Type()).Elem()
		if err := r.deeplyReplace(copyValue, originalValue, replaceMap, useGoTemplate); err != nil {
			return err
		}
		copy.Set(copyValue)

	// If it is a struct we translate each field
	case reflect.Struct:
		for i := 0; i < original.NumField(); i += 1 {
			var currentType = fmt.Sprintf("%s.%s", original.Type().Field(i).Name, original.Type().PkgPath())
			// specific case time
			if currentType == "time.Time" {
				copy.Field(i).Set(original.Field(i))
			} else if err := r.deeplyReplace(copy.Field(i), original.Field(i), replaceMap, useGoTemplate); err != nil {
				return err
			}
		}

	// If it is a slice we create a new slice and translate each element
	case reflect.Slice:
		if copy.CanSet() {
			copy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		} else {
			copyValueIntoUnexported(copy, reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		}

		for i := 0; i < original.Len(); i += 1 {
			if err := r.deeplyReplace(copy.Index(i), original.Index(i), replaceMap, useGoTemplate); err != nil {
				return err
			}
		}

	// If it is a map we create a new map and translate each value
	case reflect.Map:
		if copy.CanSet() {
			copy.Set(reflect.MakeMap(original.Type()))
		} else {
			copyValueIntoUnexported(copy, reflect.MakeMap(original.Type()))
		}
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			if originalValue.Kind() != reflect.String && originalValue.IsNil() {
				continue
			}
			// New gives us a pointer, but again we want the value
			copyValue := reflect.New(originalValue.Type()).Elem()

			if err := r.deeplyReplace(copyValue, originalValue, replaceMap, useGoTemplate); err != nil {
				return err
			}
			copy.SetMapIndex(key, copyValue)
		}

	// Otherwise we cannot traverse anywhere so this finishes the recursion
	// If it is a string translate it (yay finally we're doing what we came for)
	case reflect.String:
		strToTemplate := original.String()
		templated, err := r.Replace(strToTemplate, replaceMap, useGoTemplate)
		if err != nil {
			return err
		}
		if copy.CanSet() {
			copy.SetString(templated)
		} else {
			copyValueIntoUnexported(copy, reflect.ValueOf(templated))
		}
		return nil

	// And everything else will simply be taken from the original
	default:
		if copy.CanSet() {
			copy.Set(original)
		} else {
			copyUnexported(copy, original)
		}
	}
	return nil
}

func (r *Render) RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsv1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool) (*argoappsv1.Application, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("application template is empty ")
	}

	if len(params) == 0 {
		return tmpl, nil
	}

	original := reflect.ValueOf(tmpl)
	copy := reflect.New(original.Type()).Elem()

	if err := r.deeplyReplace(copy, original, params, useGoTemplate); err != nil {
		return nil, err
	}

	replacedTmpl := copy.Interface().(*argoappsv1.Application)

	// Add the 'resources-finalizer' finalizer if:
	// The template application doesn't have any finalizers, and:
	// a) there is no syncPolicy, or
	// b) there IS a syncPolicy, but preserveResourcesOnDeletion is set to false
	// See TestRenderTemplateParamsFinalizers in util_test.go for test-based definition of behaviour
	if (syncPolicy == nil || !syncPolicy.PreserveResourcesOnDeletion) &&
		((*replacedTmpl).ObjectMeta.Finalizers == nil || len((*replacedTmpl).ObjectMeta.Finalizers) == 0) {

		(*replacedTmpl).ObjectMeta.Finalizers = []string{"resources-finalizer.argocd.argoproj.io"}
	}

	return replacedTmpl, nil
}

var isTemplatedRegex = regexp.MustCompile(".*{{.*}}.*")

// Replace executes basic string substitution of a template with replacement values.
// remaining in the substituted template.
func (r *Render) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool) (string, error) {
	if useGoTemplate {
		template, err := template.New("").Funcs(sprigFuncMap).Parse(tmpl)
		if err != nil {
			return "", fmt.Errorf("failed to parse template %s: %w", tmpl, err)
		}

		var replacedTmplBuffer bytes.Buffer
		if err = template.Execute(&replacedTmplBuffer, replaceMap); err != nil {
			return "", fmt.Errorf("failed to execute go template %s: %w", tmpl, err)
		}

		return replacedTmplBuffer.String(), nil
	}

	if !isTemplatedRegex.MatchString(tmpl) {
		return tmpl, nil
	}

	fstTmpl := fasttemplate.New(tmpl, "{{", "}}")
	replacedTmpl := fstTmpl.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		trimmedTag := strings.TrimSpace(tag)
		replacement, ok := replaceMap[trimmedTag].(string)
		if len(trimmedTag) == 0 || !ok {
			return w.Write([]byte(fmt.Sprintf("{{%s}}", tag)))
		}
		return w.Write([]byte(replacement))
	})
	return replacedTmpl, nil
}

// Log a warning if there are unrecognized generators
func CheckInvalidGenerators(applicationSetInfo *argoappsv1.ApplicationSet) error {
	hasInvalidGenerators, invalidGenerators := invalidGenerators(applicationSetInfo)
	var errorMessage error
	if len(invalidGenerators) > 0 {
		gnames := []string{}
		for n := range invalidGenerators {
			gnames = append(gnames, n)
		}
		sort.Strings(gnames)
		aname := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators: %s"
		errorMessage = fmt.Errorf(msg, aname, strings.Join(gnames, ", "))
		log.Warnf(msg, aname, strings.Join(gnames, ", "))
	} else if hasInvalidGenerators {
		name := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators"
		errorMessage = fmt.Errorf(msg, name)
		log.Warnf(msg, name)
	}
	return errorMessage
}

// Return true if there are unknown generators specified in the application set.  If we can discover the names
// of these generators, return the names as the keys in a map
func invalidGenerators(applicationSetInfo *argoappsv1.ApplicationSet) (bool, map[string]bool) {
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

func addInvalidGeneratorNames(names map[string]bool, applicationSetInfo *argoappsv1.ApplicationSet, index int) {
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

// SanitizeName sanitizes the name in accordance with the below rules
// 1. contain no more than 253 characters
// 2. contain only lowercase alphanumeric characters, '-' or '.'
// 3. start and end with an alphanumeric character
func SanitizeName(name string) string {
	invalidDNSNameChars := regexp.MustCompile("[^-a-z0-9.]")
	maxDNSNameLength := 253

	name = strings.ToLower(name)
	name = invalidDNSNameChars.ReplaceAllString(name, "-")
	if len(name) > maxDNSNameLength {
		name = name[:maxDNSNameLength]
	}

	return strings.Trim(name, "-.")
}
