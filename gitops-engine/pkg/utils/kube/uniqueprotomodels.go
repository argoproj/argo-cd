package kube

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
)

/**
The upstream Kubernetes NewGVKParser method causes problems for Argo CD.
https://github.com/kubernetes/apimachinery/blob/eb26334eeb0f769be8f0c5665ff34713cfdec83e/pkg/util/managedfields/gvkparser.go#L73

The function fails in instances where it is probably more desirable for Argo CD to simply ignore the error and move on.
But since the upstream implementation doesn't offer the option to ignore the error, we have to mutate the input to the
function to completely avoid the case that can produce the error.

When encountering the error from NewGVKParser, we used to just set the internal GVKParser instance to nil, log the
error as info, and move on.

But Argo CD increasingly relies on the GVKParser to produce reliable diffs, especially with server-side diffing. And
we're better off with an incorrectly-initialized GVKParser than no GVKParser at all.

To understand why NewGVKParser fails, we need to understand how Kubernetes constructs its OpenAPI models.

Kubernetes contains a built-in OpenAPI document containing the `definitions` for every built-in Kubernetes API. This
document includes shared structs like APIResourceList. Some of these definitions include an
x-kubernetes-group-version-kind extension.

Aggregated APIs produce their own OpenAPI documents, which are merged with the built-in OpenAPI document. The aggregated
API documents generally include all the definitions of all the structs which are used anywhere by the API. This often
includes some of the same structs as the built-in OpenAPI document.

So when Kubernetes constructs the complete OpenAPI document (the one served at /openapi/v2), it merges the built-in
OpenAPI document with the aggregated API OpenAPI documents.

When the aggregator encounters two different definitions for the same struct (as determined by a deep compare) with the
same GVK (as determined by the value in the x-kubernetes-group-version-kind extension), it appends a `_vX` suffix to the
definition name in the OpenAPI document (where X is the count of the number of times the aggregator has seen the same
definition). Basically, it's communicating "different APIs have different opinions about the structure of structs with
this GVK, so I'm going to give them different names and let you sort it out."
https://github.com/kubernetes/kube-openapi/blob/b456828f718bab62dc3013d192665eb3d17f8fe9/pkg/aggregator/aggregator.go#L238-L279

This behavior is fine from the perspective of a typical Kubernetes API user. They download the OpenAPI document, they
see that there are two different "opinions" about the structure of a struct, and they can choose which one they want to
rely on.

But Argo CD has to be generic. We need to take the provided OpenAPI document and use it to construct a GVKParser. And
the GVKParser (reasonably) rejects the OpenAPI document if it contains two definitions for the same struct.

So we have to do some work to make the OpenAPI document palatable to the GVKParser. We have to remove the duplicate
definitions. Specifically, we take the first one and log a warning for each subsequent definition with the same GVK.

In practice, this probably generally appears when a common aggregated API was built at a time significantly before the
current Kubernetes version. The most common case is that the metrics server is built against an older version of the
Kubernetes libraries, using old versions of the structs. When the metrics server is updated to use the latest version of
the Kubernetes libraries, the problems go away, because the aggregated API and Kubernetes agree about the shape of the
struct.

Using the first encountered definition is imperfect and could result in unreliable diffs. But it's better than
constructing completely-wrong diffs due to the lack of a GVKParser.
*/

// uniqueModels is a model provider that ensures that no two models share the same gvk. Use newUniqueModels to
// initialize it and enforce uniqueness.
type uniqueModels struct {
	models map[string]proto.Schema
}

// LookupModel is public through the interface of Models. It
// returns a visitable schema from the given model name.
// Copied verbatim from here: https://github.com/kubernetes/kube-openapi/blob/b456828f718bab62dc3013d192665eb3d17f8fe9/pkg/util/proto/document.go#L322-L326
func (d *uniqueModels) LookupModel(model string) proto.Schema {
	return d.models[model]
}

// Copied verbatim from here: https://github.com/kubernetes/kube-openapi/blob/b456828f718bab62dc3013d192665eb3d17f8fe9/pkg/util/proto/document.go#L328-L337
func (d *uniqueModels) ListModels() []string {
	models := []string{}

	for model := range d.models {
		models = append(models, model)
	}

	sort.Strings(models)
	return models
}

// newUniqueModels returns a new uniqueModels instance and a list of warnings for models that share the same gvk.
func newUniqueModels(models proto.Models) (proto.Models, []schema.GroupVersionKind) {
	var taintedGVKs []schema.GroupVersionKind
	gvks := map[schema.GroupVersionKind]string{}
	um := &uniqueModels{models: map[string]proto.Schema{}}
	for _, modelName := range models.ListModels() {
		model := models.LookupModel(modelName)
		if model == nil {
			panic(fmt.Sprintf("ListModels returns a model that can't be looked-up for: %v", modelName))
		}
		gvkList := parseGroupVersionKind(model)
		gvk, wasProcessed := modelGvkWasAlreadyProcessed(model, gvks)
		if !wasProcessed {
			um.models[modelName] = model

			// Add GVKs to the map, so we can detect a duplicate GVK later.
			for _, gvk := range gvkList {
				if len(gvk.Kind) > 0 {
					gvks[gvk] = modelName
				}
			}
		} else {
			taintedGVKs = append(taintedGVKs, gvk)
		}
	}
	return um, taintedGVKs
}

// modelGvkWasAlreadyProcessed inspects a model to determine if it would trigger a duplicate GVK error. The gvks map
// holds the GVKs of all the models that have already been processed. If the model would trigger a duplicate GVK error,
// the function returns the GVK that would trigger the error and true. Otherwise, it returns an empty GVK and false.
func modelGvkWasAlreadyProcessed(model proto.Schema, gvks map[schema.GroupVersionKind]string) (schema.GroupVersionKind, bool) {
	gvkList := parseGroupVersionKind(model)
	// Not every model has a GVK extension specified. For those models, this loop will be skipped.
	for _, gvk := range gvkList {
		// The kind length check is copied from managedfields.NewGVKParser. It's unclear what edge case it's handling,
		// but the behavior of this function should match NewGVKParser.
		if len(gvk.Kind) > 0 {
			_, ok := gvks[gvk]
			if ok {
				// This is the only condition under which NewGVKParser would return a duplicate GVK error.
				return gvk, true
			}
		}
	}
	return schema.GroupVersionKind{}, false
}

// groupVersionKindExtensionKey is the key used to lookup the
// GroupVersionKind value for an object definition from the
// definition's "extensions" map.
// Copied verbatim from: https://github.com/kubernetes/apimachinery/blob/eb26334eeb0f769be8f0c5665ff34713cfdec83e/pkg/util/managedfields/gvkparser.go#L29-L32
const groupVersionKindExtensionKey = "x-kubernetes-group-version-kind"

// parseGroupVersionKind gets and parses GroupVersionKind from the extension. Returns empty if it doesn't have one.
// Copied verbatim from: https://github.com/kubernetes/apimachinery/blob/eb26334eeb0f769be8f0c5665ff34713cfdec83e/pkg/util/managedfields/gvkparser.go#L82-L128
func parseGroupVersionKind(s proto.Schema) []schema.GroupVersionKind {
	extensions := s.GetExtensions()

	gvkListResult := []schema.GroupVersionKind{}

	// Get the extensions
	gvkExtension, ok := extensions[groupVersionKindExtensionKey]
	if !ok {
		return []schema.GroupVersionKind{}
	}

	// gvk extension must be a list of at least 1 element.
	gvkList, ok := gvkExtension.([]any)
	if !ok {
		return []schema.GroupVersionKind{}
	}

	for _, gvk := range gvkList {
		// gvk extension list must be a map with group, version, and
		// kind fields
		gvkMap, ok := gvk.(map[any]any)
		if !ok {
			continue
		}
		group, ok := gvkMap["group"].(string)
		if !ok {
			continue
		}
		version, ok := gvkMap["version"].(string)
		if !ok {
			continue
		}
		kind, ok := gvkMap["kind"].(string)
		if !ok {
			continue
		}

		gvkListResult = append(gvkListResult, schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
	}

	return gvkListResult
}
