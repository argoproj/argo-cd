package kube

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kubectl/pkg/util/openapi"
)

// openAPIResources implements openapi.Resources using only the parsed OpenAPI models.
//
// kubectl's implementation (openapi.NewOpenAPIData) also keeps the decoded *openapi_v2.Document,
// only so that GetConsumes can scan its paths. The cluster cache keeps one openapi.Resources per
// cluster for as long as the cluster is cached, and the decoded document is several times larger
// than the parsed models, so retaining it costs a lot of memory for a method nothing calls.
type openAPIResources struct {
	// resources maps each GVK to the name of its model.
	resources map[schema.GroupVersionKind]string
	models    proto.Models
}

var _ openapi.Resources = &openAPIResources{}

// newOpenAPIResources indexes models by GVK. It maps GVKs to model names the same way
// openapi.NewOpenAPIData does, so LookupResource returns the same schemas.
func newOpenAPIResources(models proto.Models) *openAPIResources {
	resources := map[schema.GroupVersionKind]string{}
	for _, modelName := range models.ListModels() {
		model := models.LookupModel(modelName)
		if model == nil {
			continue
		}
		for _, gvk := range parseGroupVersionKind(model) {
			if gvk.Kind != "" {
				resources[gvk] = modelName
			}
		}
	}
	return &openAPIResources{resources: resources, models: models}
}

// LookupResource returns the schema of the model for gvk, or nil if there isn't one.
func (r *openAPIResources) LookupResource(gvk schema.GroupVersionKind) proto.Schema {
	modelName, found := r.resources[gvk]
	if !found {
		return nil
	}
	return r.models.LookupModel(modelName)
}

// GetConsumes always returns nil, because answering it requires keeping the whole OpenAPI document.
//
// kubectl's client-side apply calls GetConsumes to choose between a strategic merge patch and a
// JSON merge patch, but Argo CD's apply gets its OpenAPI data from the kubectl factory, not from
// this type. Do not pass this type to kubectl's apply Patcher: it would fall back to a JSON merge
// patch for every resource.
func (*openAPIResources) GetConsumes(schema.GroupVersionKind, string) []string {
	return nil
}
