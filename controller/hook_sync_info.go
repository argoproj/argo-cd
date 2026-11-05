package controller

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"

	cdcommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// syncInfoRevision is the only inject-sync-info field so far; new fields are added explicitly
// so hooks that opted in earlier keep receiving exactly what they asked for.
const syncInfoRevision = "revision"

// integerName matches names that look like positions regardless of size, unlike strconv.Atoi.
var integerName = regexp.MustCompile(`^[+-]?\d+$`)

// syncRevisionAnnotations returns the annotations describing the revisions of a sync operation.
// Multi-source operations get one annotation per source keyed by its 1-based position in spec.sources
// (matching --source-position), plus one keyed by its name when that name is unique and forms a
// valid annotation key.
func syncRevisionAnnotations(revision string, revisions []string, sources []v1alpha1.ApplicationSource, multiSource bool) (map[string]string, error) {
	if !multiSource {
		if revision == "" {
			return nil, errors.New("sync operation has no resolved revision")
		}
		return map[string]string{cdcommon.AnnotationSyncRevision: revision}, nil
	}
	if len(revisions) == 0 {
		return nil, errors.New("sync operation has no resolved revisions")
	}
	nameCount := make(map[string]int, len(sources))
	for _, src := range sources {
		nameCount[src.Name]++
	}
	annotations := make(map[string]string, 2*len(revisions))
	for i, r := range revisions {
		if r == "" {
			return nil, fmt.Errorf("sync operation has no resolved revision for source %d", i+1)
		}
		annotations[cdcommon.AnnotationSyncRevision+"-"+strconv.Itoa(i+1)] = r
		if i >= len(sources) || sources[i].Name == "" || nameCount[sources[i].Name] > 1 {
			continue
		}
		key := cdcommon.AnnotationSyncRevision + "-" + sources[i].Name
		// Names that look like positions would shadow another source's positional key.
		if integerName.MatchString(sources[i].Name) || len(validation.IsQualifiedName(key)) > 0 {
			continue
		}
		annotations[key] = r
	}
	return annotations, nil
}

// injectSyncInfoIntoHooks stamps operation metadata onto copies of opted-in hooks. The hook
// itself is always annotated; a Job's Pod template is annotated too so its containers can read
// the values through the Downward API. Comparison targets are untouched, so no drift is created.
func injectSyncInfoIntoHooks(hooks []*unstructured.Unstructured, revision string, revisions []string, sources []v1alpha1.ApplicationSource, multiSource bool) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, len(hooks))
	copy(result, hooks)

	var revisionAnnotations map[string]string
	for i, hook := range hooks {
		value, ok := hook.GetAnnotations()[cdcommon.AnnotationInjectSyncInfo]
		if !ok {
			continue
		}
		toInject := map[string]string{}
		for field := range strings.SplitSeq(value, ",") {
			switch strings.TrimSpace(field) {
			case "":
			case syncInfoRevision:
				if revisionAnnotations == nil {
					var err error
					if revisionAnnotations, err = syncRevisionAnnotations(revision, revisions, sources, multiSource); err != nil {
						return nil, fmt.Errorf("cannot inject %s into %s %q: %w", syncInfoRevision, hook.GetKind(), hookName(hook), err)
					}
				}
				maps.Copy(toInject, revisionAnnotations)
			default:
				return nil, fmt.Errorf("unsupported %s field %q on %s %q", cdcommon.AnnotationInjectSyncInfo, strings.TrimSpace(field), hook.GetKind(), hookName(hook))
			}
		}
		if len(toInject) == 0 {
			continue
		}

		hookCopy := hook.DeepCopy()
		annotations := hookCopy.GetAnnotations()
		maps.Copy(annotations, toInject)
		hookCopy.SetAnnotations(annotations)

		if hookCopy.GetAPIVersion() == "batch/v1" && hookCopy.GetKind() == "Job" {
			// A null template metadata is valid YAML but cannot be traversed or set into.
			if metadata, found, _ := unstructured.NestedFieldNoCopy(hookCopy.Object, "spec", "template", "metadata"); found && metadata == nil {
				unstructured.RemoveNestedField(hookCopy.Object, "spec", "template", "metadata")
			}
			podAnnotations, _, err := unstructured.NestedNullCoercingStringMap(hookCopy.Object, "spec", "template", "metadata", "annotations")
			if err != nil {
				return nil, fmt.Errorf("failed to read Pod template annotations of Job %q: %w", hookName(hook), err)
			}
			if podAnnotations == nil {
				podAnnotations = make(map[string]string, len(toInject))
			}
			maps.Copy(podAnnotations, toInject)
			if err := unstructured.SetNestedStringMap(hookCopy.Object, podAnnotations, "spec", "template", "metadata", "annotations"); err != nil {
				return nil, fmt.Errorf("failed to set Pod template annotations of Job %q: %w", hookName(hook), err)
			}
		}
		result[i] = hookCopy
	}
	return result, nil
}

// hookName identifies a hook in messages; generateName hooks have no name until they are created.
func hookName(hook *unstructured.Unstructured) string {
	if name := hook.GetName(); name != "" {
		return name
	}
	if prefix := hook.GetGenerateName(); prefix != "" {
		return prefix + "*"
	}
	return ""
}
