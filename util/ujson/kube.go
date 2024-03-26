package ujson

import (
	"strconv"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KubeJson struct {
	data       []byte
	apiversion []byte
	kind       []byte
	namespace  []byte
	name       []byte
	res        *MatchResult
	opt        *MatchOptions
}

func NewKubeJson(data []byte) (*KubeJson, error) {
	k := &KubeJson{
		data:       data,
		apiversion: []byte(""),
		kind:       []byte(""),
		namespace:  []byte(""),
		name:       []byte(""),
		res:        &MatchResult{},
		opt: &MatchOptions{
			IgnoreCase:       false,
			QuitIfNoCallback: true,
		},
	}
	callbacks := []*MatchCallback{
		{
			paths: []string{"\"metadata\"", "\"name\""},
			cb: func(paths [][]byte, value []byte) error {
				k.name = value
				return nil
			},
		},
		{
			paths: []string{"\"metadata\"", "\"namespace\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				k.namespace = value
				return nil
			},
		},
		{
			paths: []string{"\"kind\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				k.kind = value
				return nil
			},
		},
		{
			paths: []string{"\"apiVersion\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				k.apiversion = value
				return nil
			},
		},
	}
	// save start time
	err := Match(data, k.opt, k.res, callbacks...)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (k *KubeJson) IsEmpty() bool {
	return k.res.Count == 0
}

func (k *KubeJson) GetAPIVersion() string {
	return unquote(string(k.apiversion))
}

func (k *KubeJson) GetKind() string {
	return unquote(string(k.kind))
}

func (k *KubeJson) GetNamespace() string {
	return unquote(string(k.namespace))
}

func (k *KubeJson) GetName() string {
	return unquote(string(k.name))
}

func (k *KubeJson) GroupVersionKind() schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(k.GetAPIVersion())
	if err != nil {
		return schema.GroupVersionKind{}
	}
	gvk := gv.WithKind(k.GetKind())
	return gvk
}

func GetResourceKey(obj *KubeJson) kube.ResourceKey {
	gvk := obj.GroupVersionKind()
	return kube.NewResourceKey(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
}

func unquote(s string) string {
	trimmedStr := strings.TrimSpace(s)
	if len(trimmedStr) == 0 {
		return ""
	}
	if trimmedStr[0] == '"' && trimmedStr[len(trimmedStr)-1] == '"' {
		unquotedStr, err := strconv.Unquote(trimmedStr)
		if err != nil {
			return trimmedStr
		}
		return unquotedStr
	}
	return trimmedStr
}
