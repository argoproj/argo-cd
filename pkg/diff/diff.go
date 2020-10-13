/*
The package provide functions that allows to compare set of Kubernetes resources using the logic equivalent to
`kubectl diff`.
*/
package diff

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/api/endpoints"
	"k8s.io/kubernetes/pkg/apis/core"
	v1 "k8s.io/kubernetes/pkg/apis/core/v1"

	jsonutil "github.com/argoproj/gitops-engine/pkg/utils/json"
	kubescheme "github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"
)

const couldNotMarshalErrMsg = "Could not unmarshal to object of type %s: %v"

// Holds diffing settings
type DiffOptions struct {
	// If set to true then differences caused by aggregated roles in RBAC resources are ignored.
	IgnoreAggregatedRoles bool `json:"ignoreAggregatedRoles,omitempty"`
}

// Holds diffing result of two resources
type DiffResult struct {
	// Modified is set to true if resources are not matching
	Modified bool
	// Contains YAML representation of a live resource with applied normalizations
	NormalizedLive []byte
	// Contains "expected" YAML representation of a live resource
	PredictedLive []byte
}

// Holds result of two resources sets comparison
type DiffResultList struct {
	Diffs    []DiffResult
	Modified bool
}

type noopNormalizer struct {
}

func (n *noopNormalizer) Normalize(un *unstructured.Unstructured) error {
	return nil
}

// Normalizer updates resource before comparing it
type Normalizer interface {
	Normalize(un *unstructured.Unstructured) error
}

// GetNoopNormalizer returns normalizer that does not apply any resource modifications
func GetNoopNormalizer() Normalizer {
	return &noopNormalizer{}
}

// Returns the default diff options
func GetDefaultDiffOptions() DiffOptions {
	return DiffOptions{
		IgnoreAggregatedRoles: false,
	}
}

// Diff performs a diff on two unstructured objects. If the live object happens to have a
// "kubectl.kubernetes.io/last-applied-configuration", then perform a three way diff.
func Diff(config, live *unstructured.Unstructured, normalizer Normalizer, options DiffOptions) (*DiffResult, error) {
	if config != nil {
		config = remarshal(config)
		Normalize(config, normalizer, options)
	}
	if live != nil {
		live = remarshal(live)
		Normalize(live, normalizer, options)
	}
	orig := GetLastAppliedConfigAnnotation(live)
	if orig != nil && config != nil {
		Normalize(orig, normalizer, options)
		dr, err := ThreeWayDiff(orig, config, live)
		if err == nil {
			return dr, nil
		}
		log.Debugf("three-way diff calculation failed: %v. Falling back to two-way diff", err)
	}
	return TwoWayDiff(config, live)
}

// TwoWayDiff performs a three-way diff and uses specified config as a recently applied config
func TwoWayDiff(config, live *unstructured.Unstructured) (*DiffResult, error) {
	if live != nil && config != nil {
		return ThreeWayDiff(config, config.DeepCopy(), live)
	} else if live != nil {
		liveData, err := json.Marshal(live)
		if err != nil {
			return nil, err
		}
		return &DiffResult{Modified: false, NormalizedLive: liveData, PredictedLive: []byte("null")}, nil
	} else if config != nil {
		predictedLiveData, err := json.Marshal(config.Object)
		if err != nil {
			return nil, err
		}
		return &DiffResult{Modified: true, NormalizedLive: []byte("null"), PredictedLive: predictedLiveData}, nil
	} else {
		return nil, errors.New("both live and config are null objects")
	}
}

// applyPatch executes kubernetes server side patch:
// uses corresponding data structure, applies appropriate defaults and executes strategic merge patch
func applyPatch(liveBytes []byte, patchBytes []byte, newVersionedObject func() (runtime.Object, error)) ([]byte, []byte, error) {
	predictedLive, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}
	predictedLiveBytes, err := strategicpatch.StrategicMergePatch(liveBytes, patchBytes, predictedLive)
	if err != nil {
		return nil, nil, err
	}

	if err = json.Unmarshal(predictedLiveBytes, &predictedLive); err == nil {
		kubescheme.Scheme.Default(predictedLive)
		predictedLiveBytes, err = json.Marshal(predictedLive)
		if err != nil {
			return nil, nil, err
		}
	}

	live, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}

	if err = json.Unmarshal(liveBytes, live); err == nil {
		kubescheme.Scheme.Default(live)
		liveBytes, err = json.Marshal(live)
		if err != nil {
			return nil, nil, err
		}
	}

	return liveBytes, predictedLiveBytes, nil
}

// ThreeWayDiff performs a diff with the understanding of how to incorporate the
// last-applied-configuration annotation in the diff.
// Inputs are assumed to be stripped of type information
func ThreeWayDiff(orig, config, live *unstructured.Unstructured) (*DiffResult, error) {
	orig = removeNamespaceAnnotation(orig)
	config = removeNamespaceAnnotation(config)

	// 1. calculate a 3-way merge patch
	patchBytes, newVersionedObject, err := threeWayMergePatch(orig, config, live)
	if err != nil {
		return nil, err
	}

	// 2. get expected live object by applying the patch against the live object
	liveBytes, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}

	var predictedLiveBytes []byte
	if newVersionedObject != nil {
		liveBytes, predictedLiveBytes, err = applyPatch(liveBytes, patchBytes, newVersionedObject)
		if err != nil {
			return nil, err
		}
	} else {
		predictedLiveBytes, err = jsonpatch.MergePatch(liveBytes, patchBytes)
		if err != nil {
			return nil, err
		}
	}

	predictedLive := &unstructured.Unstructured{}
	err = json.Unmarshal(predictedLiveBytes, predictedLive)
	if err != nil {
		return nil, err
	}

	// 3. compare live and expected live object
	dr := DiffResult{
		PredictedLive:  predictedLiveBytes,
		NormalizedLive: liveBytes,
		Modified:       string(predictedLiveBytes) != string(liveBytes),
	}
	return &dr, nil
}

// stripTypeInformation strips any type information (e.g. float64 vs. int) from the unstructured
// object by remarshalling the object. This is important for diffing since it will cause godiff
// to report a false difference.
func stripTypeInformation(un *unstructured.Unstructured) *unstructured.Unstructured {
	unBytes, err := json.Marshal(un)
	if err != nil {
		panic(err)
	}
	var newUn unstructured.Unstructured
	err = json.Unmarshal(unBytes, &newUn)
	if err != nil {
		panic(err)
	}
	return &newUn
}

// removeNamespaceAnnotation remove the namespace and an empty annotation map from the metadata.
// The namespace field is present in live (namespaced) objects, but not necessarily present in
// config or last-applied. This results in a diff which we don't care about. We delete the two so
// that the diff is more relevant.
func removeNamespaceAnnotation(orig *unstructured.Unstructured) *unstructured.Unstructured {
	orig = orig.DeepCopy()
	if metadataIf, ok := orig.Object["metadata"]; ok {
		metadata := metadataIf.(map[string]interface{})
		delete(metadata, "namespace")
		if annotationsIf, ok := metadata["annotations"]; ok {
			shouldDelete := false
			if annotationsIf == nil {
				shouldDelete = true
			} else {
				annotation := annotationsIf.(map[string]interface{})
				if len(annotation) == 0 {
					shouldDelete = true
				}
			}
			if shouldDelete {
				delete(metadata, "annotations")
			}
		}
	}
	return orig
}

// StatefulSet requires special handling since it embeds PersistentVolumeClaim resource.
// K8S API server applies additional default field which we cannot reproduce on client side.
// So workaround is to remove all "defaulted" fields from 'volumeClaimTemplates' of live resource.
func statefulSetWorkaround(orig, live *unstructured.Unstructured) *unstructured.Unstructured {
	origTemplate, ok, err := unstructured.NestedSlice(orig.Object, "spec", "volumeClaimTemplates")
	if !ok || err != nil {
		return live
	}

	liveTemplate, ok, err := unstructured.NestedSlice(live.Object, "spec", "volumeClaimTemplates")
	if !ok || err != nil {
		return live
	}
	live = live.DeepCopy()

	_ = unstructured.SetNestedField(live.Object, jsonutil.RemoveListFields(origTemplate, liveTemplate), "spec", "volumeClaimTemplates")
	return live
}

func threeWayMergePatch(orig, config, live *unstructured.Unstructured) ([]byte, func() (runtime.Object, error), error) {
	origBytes, err := json.Marshal(orig.Object)
	if err != nil {
		return nil, nil, err
	}
	configBytes, err := json.Marshal(config.Object)
	if err != nil {
		return nil, nil, err
	}

	if versionedObject, err := scheme.Scheme.New(orig.GroupVersionKind()); err == nil {
		gk := orig.GroupVersionKind().GroupKind()
		if (gk.Group == "apps" || gk.Group == "extensions") && gk.Kind == "StatefulSet" {
			live = statefulSetWorkaround(orig, live)
		}

		liveBytes, err := json.Marshal(live.Object)
		if err != nil {
			return nil, nil, err
		}

		lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
		if err != nil {
			return nil, nil, err
		}
		patch, err := strategicpatch.CreateThreeWayMergePatch(origBytes, configBytes, liveBytes, lookupPatchMeta, true)
		if err != nil {
			return nil, nil, err
		}
		newVersionedObject := func() (runtime.Object, error) {
			return scheme.Scheme.New(orig.GroupVersionKind())
		}
		return patch, newVersionedObject, nil
	} else {
		// Remove defaulted fields from the live object.
		// This subtracts any extra fields in the live object which are not present in last-applied-configuration.
		live = &unstructured.Unstructured{Object: jsonutil.RemoveMapFields(orig.Object, live.Object)}

		liveBytes, err := json.Marshal(live.Object)
		if err != nil {
			return nil, nil, err
		}

		patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(origBytes, configBytes, liveBytes)
		if err != nil {
			return nil, nil, err
		}
		return patch, nil, nil
	}
}

func GetLastAppliedConfigAnnotation(live *unstructured.Unstructured) *unstructured.Unstructured {
	if live == nil {
		return nil
	}
	annots := live.GetAnnotations()
	if annots == nil {
		return nil
	}
	lastAppliedStr, ok := annots[corev1.LastAppliedConfigAnnotation]
	if !ok {
		return nil
	}
	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(lastAppliedStr), &obj)
	if err != nil {
		log.Warnf("Failed to unmarshal %s in %s", corev1.LastAppliedConfigAnnotation, live.GetName())
		return nil
	}
	return &obj
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(configArray, liveArray []*unstructured.Unstructured, normalizer Normalizer, options DiffOptions) (*DiffResultList, error) {
	numItems := len(configArray)
	if len(liveArray) != numItems {
		return nil, fmt.Errorf("left and right arrays have mismatched lengths")
	}

	diffResultList := DiffResultList{
		Diffs: make([]DiffResult, numItems),
	}
	for i := 0; i < numItems; i++ {
		config := configArray[i]
		live := liveArray[i]
		diffRes, err := Diff(config, live, normalizer, options)
		if err != nil {
			return nil, err
		}
		diffResultList.Diffs[i] = *diffRes
		if diffRes.Modified {
			diffResultList.Modified = true
		}
	}
	return &diffResultList, nil
}

func Normalize(un *unstructured.Unstructured, normalizer Normalizer, options DiffOptions) {
	if un == nil {
		return
	}

	// creationTimestamp is sometimes set to null in the config when exported (e.g. SealedSecrets)
	// Removing the field allows a cleaner diff.
	unstructured.RemoveNestedField(un.Object, "metadata", "creationTimestamp")

	gvk := un.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "Secret" {
		NormalizeSecret(un)
	} else if gvk.Group == "rbac.authorization.k8s.io" && (gvk.Kind == "ClusterRole" || gvk.Kind == "Role") {
		normalizeRole(un, options)
	} else if gvk.Group == "" && gvk.Kind == "Endpoints" {
		normalizeEndpoint(un)
	}

	if normalizer != nil {
		err := normalizer.Normalize(un)
		if err != nil {
			log.Warnf("Failed to normalize %s/%s/%s: %v", un.GroupVersionKind(), un.GetNamespace(), un.GetName(), err)
		}
	}
}

// NormalizeSecret mutates the supplied object and encodes stringData to data, and converts nils to
// empty strings. If the object is not a secret, or is an invalid secret, then returns the same object.
func NormalizeSecret(un *unstructured.Unstructured) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return
	}
	var secret corev1.Secret
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &secret)
	if err != nil {
		return
	}
	// We normalize nils to empty string to handle: https://github.com/argoproj/argo-cd/issues/943
	for k, v := range secret.Data {
		if len(v) == 0 {
			secret.Data[k] = []byte("")
		}
	}
	if len(secret.StringData) > 0 {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		for k, v := range secret.StringData {
			secret.Data[k] = []byte(v)
		}
		delete(un.Object, "stringData")
	}
	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&secret)
	if err != nil {
		log.Warnf("object unable to convert from secret: %v", err)
		return
	}
	if secret.Data != nil {
		err = unstructured.SetNestedField(un.Object, newObj["data"], "data")
		if err != nil {
			log.Warnf("failed to set secret.data: %v", err)
			return
		}
	}
}

// normalizeEndpoint normalizes endpoint meaning that EndpointSubsets are sorted lexicographically
func normalizeEndpoint(un *unstructured.Unstructured) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Endpoints" {
		return
	}
	var ep corev1.Endpoints
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &ep)
	if err != nil {
		return
	}
	var coreEp core.Endpoints
	err = v1.Convert_v1_Endpoints_To_core_Endpoints(&ep, &coreEp, nil)
	if err != nil {
		log.Warnf("Could not convert from v1 to core endpoint type %s: %v", gvk, err)
		return
	}

	endpoints.SortSubsets(coreEp.Subsets)

	err = v1.Convert_core_Endpoints_To_v1_Endpoints(&coreEp, &ep, nil)
	if err != nil {
		log.Warnf("Could not convert from core to vi endpoint type %s: %v", gvk, err)
		return
	}
	un.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&ep)
	if err != nil {
		log.Warnf(couldNotMarshalErrMsg, gvk, err)
		return
	}
}

// normalizeRole mutates the supplied Role/ClusterRole and sets rules to null if it is an empty list or an aggregated role
func normalizeRole(un *unstructured.Unstructured, options DiffOptions) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "rbac.authorization.k8s.io" || (gvk.Kind != "Role" && gvk.Kind != "ClusterRole") {
		return
	}

	// Check whether the role we're checking is an aggregation role. If it is, we ignore any differences in rules.
	if options.IgnoreAggregatedRoles {
		aggrIf, ok := un.Object["aggregationRule"]
		if ok {
			_, ok = aggrIf.(map[string]interface{})
			if !ok {
				log.Infof("Malformed aggregrationRule in resource '%s', won't modify.", un.GetName())
			} else {
				un.Object["rules"] = nil
			}
		}
	}

	rulesIf, ok := un.Object["rules"]
	if !ok {
		return
	}
	rules, ok := rulesIf.([]interface{})
	if !ok {
		return
	}
	if rules != nil && len(rules) == 0 {
		un.Object["rules"] = nil
	}

}

// CreateTwoWayMergePatch is a helper to construct a two-way merge patch from objects (instead of bytes)
func CreateTwoWayMergePatch(orig, new, dataStruct interface{}) ([]byte, bool, error) {
	origBytes, err := json.Marshal(orig)
	if err != nil {
		return nil, false, err
	}
	newBytes, err := json.Marshal(new)
	if err != nil {
		return nil, false, err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(origBytes, newBytes, dataStruct)
	if err != nil {
		return nil, false, err
	}
	return patch, string(patch) != "{}", nil
}

// HideSecretData replaces secret data values in specified target, live secrets and in last applied configuration of live secret with stars. Also preserves differences between
// target, live and last applied config values. E.g. if all three are equal the values would be replaced with same number of stars. If all the are different then number of stars
// in replacement should be different.
func HideSecretData(target *unstructured.Unstructured, live *unstructured.Unstructured) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	var orig *unstructured.Unstructured
	if live != nil {
		orig = GetLastAppliedConfigAnnotation(live)
		live = live.DeepCopy()
	}
	if target != nil {
		target = target.DeepCopy()
	}

	keys := map[string]bool{}
	for _, obj := range []*unstructured.Unstructured{target, live, orig} {
		if obj == nil {
			continue
		}
		NormalizeSecret(obj)
		if data, found, err := unstructured.NestedMap(obj.Object, "data"); found && err == nil {
			for k := range data {
				keys[k] = true
			}
		}
	}

	for k := range keys {
		// we use "+" rather than the more common "*"
		nextReplacement := "++++++++"
		valToReplacement := make(map[string]string)
		for _, obj := range []*unstructured.Unstructured{target, live, orig} {
			var data map[string]interface{}
			if obj != nil {
				var err error
				data, _, err = unstructured.NestedMap(obj.Object, "data")
				if err != nil {
					return nil, nil, err
				}
			}
			if data == nil {
				data = make(map[string]interface{})
			}
			valData, ok := data[k]
			if !ok {
				continue
			}
			val := toString(valData)
			replacement, ok := valToReplacement[val]
			if !ok {
				replacement = nextReplacement
				nextReplacement = nextReplacement + "++++"
				valToReplacement[val] = replacement
			}
			data[k] = replacement
			err := unstructured.SetNestedField(obj.Object, data, "data")
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if live != nil && orig != nil {
		annotations := live.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		lastAppliedData, err := json.Marshal(orig)
		if err != nil {
			return nil, nil, err
		}
		annotations[corev1.LastAppliedConfigAnnotation] = string(lastAppliedData)
		live.SetAnnotations(annotations)
	}
	return target, live, nil
}

func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%s", val)
}

// remarshal checks resource kind and version and re-marshal using corresponding struct custom marshaller.
// This ensures that expected resource state is formatter same as actual resource state in kubernetes
// and allows to find differences between actual and target states more accurately.
// Remarshalling also strips any type information (e.g. float64 vs. int) from the unstructured
// object. This is important for diffing since it will cause godiff to report a false difference.
func remarshal(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj = stripTypeInformation(obj)
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	gvk := obj.GroupVersionKind()
	item, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		// this is common. the scheme is not registered
		log.Debugf("Could not create new object of type %s: %v", gvk, err)
		return obj
	}
	// This will drop any omitempty fields, perform resource conversion etc...
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	err = json.Unmarshal(data, &unmarshalledObj)
	if err != nil {
		// User may have specified an invalid spec in git. Return original object
		log.Debugf(couldNotMarshalErrMsg, gvk, err)
		return obj
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		log.Warnf(couldNotMarshalErrMsg, gvk, err)
		return obj
	}
	// remove all default values specified by custom formatter (e.g. creationTimestamp)
	unstrBody = jsonutil.RemoveMapFields(obj.Object, unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}
}
