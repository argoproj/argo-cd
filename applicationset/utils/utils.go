package utils

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unsafe"

	"github.com/Masterminds/sprig/v3"
	"github.com/gosimple/slug"
	"github.com/valyala/fasttemplate"
	"sigs.k8s.io/yaml"

	log "github.com/sirupsen/logrus"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/glob"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
	sprigFuncMap["normalize"] = SanitizeName
	sprigFuncMap["slugify"] = SlugifyName
	sprigFuncMap["toYaml"] = toYAML
	sprigFuncMap["fromYaml"] = fromYAML
	sprigFuncMap["fromYamlArray"] = fromYAMLArray
}

type Renderer interface {
	RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsv1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*argoappsv1.Application, error)
	Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error)
}

type Render struct{}

func IsNamespaceAllowed(namespaces []string, namespace string) bool {
	return glob.MatchStringInList(namespaces, namespace, glob.REGEXP)
}

func copyValueIntoUnexported(destination, value reflect.Value) {
	reflect.NewAt(destination.Type(), unsafe.Pointer(destination.UnsafeAddr())).
		Elem().
		Set(value)
}

func copyUnexported(copy, original reflect.Value) {
	unexported := reflect.NewAt(original.Type(), unsafe.Pointer(original.UnsafeAddr())).Elem()
	copyValueIntoUnexported(copy, unexported)
}

func IsJSONStr(str string) bool {
	str = strings.TrimSpace(str)
	return len(str) > 0 && str[0] == '{'
}

func ConvertYAMLToJSON(str string) (string, error) {
	if !IsJSONStr(str) {
		jsonStr, err := yaml.YAMLToJSON([]byte(str))
		if err != nil {
			return str, err
		}
		return string(jsonStr), nil
	}
	return str, nil
}

// This function is in charge of searching all String fields of the object recursively and apply templating
// thanks to https://gist.github.com/randallmlough/1fd78ec8a1034916ca52281e3b886dc7
func (r *Render) deeplyReplace(copy, original reflect.Value, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) error {
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
		if err := r.deeplyReplace(copy.Elem(), originalValue, replaceMap, useGoTemplate, goTemplateOptions); err != nil {
			// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
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

		if originalValue.IsValid() {
			reflectType := originalValue.Type()

			reflectValue := reflect.New(reflectType)

			copyValue := reflectValue.Elem()
			if err := r.deeplyReplace(copyValue, originalValue, replaceMap, useGoTemplate, goTemplateOptions); err != nil {
				// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
				return err
			}
			copy.Set(copyValue)
		}

	// If it is a struct we translate each field
	case reflect.Struct:
		for i := 0; i < original.NumField(); i += 1 {
			currentType := fmt.Sprintf("%s.%s", original.Type().Field(i).Name, original.Type().PkgPath())
			// specific case time
			if currentType == "time.Time" {
				copy.Field(i).Set(original.Field(i))
			} else if currentType == "Raw.k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1" || currentType == "Raw.k8s.io/apimachinery/pkg/runtime" {
				var unmarshaled interface{}
				originalBytes := original.Field(i).Bytes()
				convertedToJson, err := ConvertYAMLToJSON(string(originalBytes))
				if err != nil {
					return fmt.Errorf("error while converting template to json %q: %w", convertedToJson, err)
				}
				err = json.Unmarshal([]byte(convertedToJson), &unmarshaled)
				if err != nil {
					return fmt.Errorf("failed to unmarshal JSON field: %w", err)
				}
				jsonOriginal := reflect.ValueOf(&unmarshaled)
				jsonCopy := reflect.New(jsonOriginal.Type()).Elem()
				err = r.deeplyReplace(jsonCopy, jsonOriginal, replaceMap, useGoTemplate, goTemplateOptions)
				if err != nil {
					return fmt.Errorf("failed to deeply replace JSON field contents: %w", err)
				}
				jsonCopyInterface := jsonCopy.Interface().(*interface{})
				data, err := json.Marshal(jsonCopyInterface)
				if err != nil {
					return fmt.Errorf("failed to marshal templated JSON field: %w", err)
				}
				copy.Field(i).Set(reflect.ValueOf(data))
			} else if err := r.deeplyReplace(copy.Field(i), original.Field(i), replaceMap, useGoTemplate, goTemplateOptions); err != nil {
				// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
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
			if err := r.deeplyReplace(copy.Index(i), original.Index(i), replaceMap, useGoTemplate, goTemplateOptions); err != nil {
				// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
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
			if originalValue.Kind() != reflect.String && isNillable(originalValue) && originalValue.IsNil() {
				continue
			}
			// New gives us a pointer, but again we want the value
			copyValue := reflect.New(originalValue.Type()).Elem()

			if err := r.deeplyReplace(copyValue, originalValue, replaceMap, useGoTemplate, goTemplateOptions); err != nil {
				// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
				return err
			}

			// Keys can be templated as well as values (e.g. to template something into an annotation).
			if key.Kind() == reflect.String {
				templatedKey, err := r.Replace(key.String(), replaceMap, useGoTemplate, goTemplateOptions)
				if err != nil {
					// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
					return err
				}
				key = reflect.ValueOf(templatedKey)
			}

			copy.SetMapIndex(key, copyValue)
		}

	// Otherwise we cannot traverse anywhere so this finishes the recursion
	// If it is a string translate it (yay finally we're doing what we came for)
	case reflect.String:
		strToTemplate := original.String()
		templated, err := r.Replace(strToTemplate, replaceMap, useGoTemplate, goTemplateOptions)
		if err != nil {
			// Not wrapping the error, since this is a recursive function. Avoids excessively long error messages.
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

// isNillable returns true if the value is something which may be set to nil. This function is meant to guard against a
// panic from calling IsNil on a non-pointer type.
func isNillable(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return true
	}
	return false
}

func (r *Render) RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsv1.ApplicationSetSyncPolicy, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*argoappsv1.Application, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("application template is empty")
	}

	if len(params) == 0 {
		return tmpl, nil
	}

	original := reflect.ValueOf(tmpl)
	copy := reflect.New(original.Type()).Elem()

	if err := r.deeplyReplace(copy, original, params, useGoTemplate, goTemplateOptions); err != nil {
		return nil, err
	}

	replacedTmpl := copy.Interface().(*argoappsv1.Application)

	// Add the 'resources-finalizer' finalizer if:
	// The template application doesn't have any finalizers, and:
	// a) there is no syncPolicy, or
	// b) there IS a syncPolicy, but preserveResourcesOnDeletion is set to false
	// See TestRenderTemplateParamsFinalizers in util_test.go for test-based definition of behaviour
	if (syncPolicy == nil || !syncPolicy.PreserveResourcesOnDeletion) &&
		len(replacedTmpl.ObjectMeta.Finalizers) == 0 {
		replacedTmpl.ObjectMeta.Finalizers = []string{"resources-finalizer.argocd.argoproj.io"}
	}

	return replacedTmpl, nil
}

func (r *Render) RenderGeneratorParams(gen *argoappsv1.ApplicationSetGenerator, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (*argoappsv1.ApplicationSetGenerator, error) {
	if gen == nil {
		return nil, fmt.Errorf("generator is empty")
	}

	if len(params) == 0 {
		return gen, nil
	}

	original := reflect.ValueOf(gen)
	copy := reflect.New(original.Type()).Elem()

	if err := r.deeplyReplace(copy, original, params, useGoTemplate, goTemplateOptions); err != nil {
		return nil, fmt.Errorf("failed to replace parameters in generator: %w", err)
	}

	replacedGen := copy.Interface().(*argoappsv1.ApplicationSetGenerator)

	return replacedGen, nil
}

var isTemplatedRegex = regexp.MustCompile(".*{{.*}}.*")

// Replace executes basic string substitution of a template with replacement values.
// remaining in the substituted template.
func (r *Render) Replace(tmpl string, replaceMap map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (string, error) {
	if useGoTemplate {
		template, err := template.New("").Funcs(sprigFuncMap).Parse(tmpl)
		if err != nil {
			return "", fmt.Errorf("failed to parse template %s: %w", tmpl, err)
		}
		for _, option := range goTemplateOptions {
			template = template.Option(option)
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

	fstTmpl, err := fasttemplate.NewTemplate(tmpl, "{{", "}}")
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}
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

// SlugifyName generates a URL-friendly slug from the provided name and additional options.
// The slug is generated in accordance with the following rules:
// 1. The generated slug will be URL-safe and suitable for use in URLs.
// 2. The maximum length of the slug can be specified using the `maxSize` argument.
// 3. Smart truncation can be enabled or disabled using the `EnableSmartTruncate` argument.
// 4. The input name can be any string value that needs to be converted into a slug.
//
// Args:
// - args: A variadic number of arguments where:
//   - The first argument (if provided) is an integer specifying the maximum length of the slug.
//   - The second argument (if provided) is a boolean indicating whether smart truncation is enabled.
//   - The last argument (if provided) is the input name that needs to be slugified.
//     If no name is provided, an empty string will be used.
//
// Returns:
// - string: The generated URL-friendly slug based on the input name and options.
func SlugifyName(args ...interface{}) string {
	// Default values for arguments
	maxSize := 50
	EnableSmartTruncate := true
	name := ""

	// Process the arguments
	for idx, arg := range args {
		switch idx {
		case len(args) - 1:
			name = arg.(string)
		case 0:
			maxSize = arg.(int)
		case 1:
			EnableSmartTruncate = arg.(bool)
		default:
			log.Errorf("Bad 'slugify' arguments.")
		}
	}

	sanitizedName := SanitizeName(name)

	// Configure slug generation options
	slug.EnableSmartTruncate = EnableSmartTruncate
	slug.MaxLength = maxSize

	// Generate the slug from the input name
	urlSlug := slug.Make(sanitizedName)

	return urlSlug
}

func getTlsConfigWithCACert(scmRootCAPath string, caCerts []byte) *tls.Config {
	tlsConfig := &tls.Config{}

	if scmRootCAPath != "" {
		_, err := os.Stat(scmRootCAPath)
		if os.IsNotExist(err) {
			log.Errorf("scmRootCAPath '%s' specified does not exist: %s", scmRootCAPath, err)
			return tlsConfig
		}
		rootCA, err := os.ReadFile(scmRootCAPath)
		if err != nil {
			log.Errorf("error reading certificate from file '%s', proceeding without custom rootCA : %s", scmRootCAPath, err)
			return tlsConfig
		}
		caCerts = append(caCerts, rootCA...)
	}

	if len(caCerts) > 0 {
		certPool := x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(caCerts)
		if !ok {
			log.Errorf("failed to append certificates from PEM: proceeding without custom rootCA")
		} else {
			tlsConfig.RootCAs = certPool
		}
	}
	return tlsConfig
}

func GetTlsConfig(scmRootCAPath string, insecure bool, caCerts []byte) *tls.Config {
	tlsConfig := getTlsConfigWithCACert(scmRootCAPath, caCerts)

	if insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	return tlsConfig
}
