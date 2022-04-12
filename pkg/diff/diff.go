/*
The package provide functions that allows to compare set of Kubernetes resources using the logic equivalent to
`kubectl diff`.
*/
package diff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/argoproj/gitops-engine/internal/kubernetes_vendor/pkg/api/v1/endpoints"
	jsonutil "github.com/argoproj/gitops-engine/pkg/utils/json"
	kubescheme "github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"
)

const couldNotMarshalErrMsg = "Could not unmarshal to object of type %s: %v"

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

// Diff performs a diff on two unstructured objects. If the live object happens to have a
// "kubectl.kubernetes.io/last-applied-configuration", then perform a three way diff.
func Diff(config, live *unstructured.Unstructured, opts ...Option) (*DiffResult, error) {
	o := applyOptions(opts)
	if config != nil {
		config = remarshal(config, o)
		Normalize(config, opts...)
	}
	if live != nil {
		live = remarshal(live, o)
		Normalize(live, opts...)
	}
	orig, err := GetLastAppliedConfigAnnotation(live)
	if err != nil {
		o.log.V(1).Info(fmt.Sprintf("Failed to get last applied configuration: %v", err))
	} else {
		if orig != nil && config != nil {
			Normalize(orig, opts...)
			dr, err := ThreeWayDiff(orig, config, live)
			if err == nil {
				return dr, nil
			}
			o.log.V(1).Info(fmt.Sprintf("three-way diff calculation failed: %v. Falling back to two-way diff", err))
		}
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

// generateSchemeDefaultPatch runs the scheme default functions on the given parameter, and
// return a patch representing the delta vs the origin parameter object.
func generateSchemeDefaultPatch(kubeObj runtime.Object) ([]byte, error) {

	// 1) Call scheme defaulter functions on a clone of our k8s resource object
	patched := kubeObj.DeepCopyObject()
	kubescheme.Scheme.Default(patched)

	// 2) Compare the original object (pre-defaulter funcs) with patched object (post-default funcs),
	// and generate a patch that can be applied against the original
	patch, success, err := CreateTwoWayMergePatch(kubeObj, patched, kubeObj.DeepCopyObject())

	// Ignore empty patch: this only means that kubescheme.Scheme.Default(...) made no changes.
	if string(patch) == "{}" && err == nil {
		success = true
	}
	if err != nil || !success {
		if err == nil && !success {
			err = errors.New("empty result")
		}
		return nil, err
	}

	return patch, err
}

// applyPatch executes kubernetes server side patch:
// uses corresponding data structure, applies appropriate defaults and executes strategic merge patch
func applyPatch(liveBytes []byte, patchBytes []byte, newVersionedObject func() (runtime.Object, error)) ([]byte, []byte, error) {

	// Construct an empty instance of the object we are applying a patch against
	predictedLive, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}

	// Apply the patchBytes patch against liveBytes, using predictedLive to indicate the k8s data type
	predictedLiveBytes, err := strategicpatch.StrategicMergePatch(liveBytes, patchBytes, predictedLive)
	if err != nil {
		return nil, nil, err
	}

	// Unmarshal predictedLiveBytes into predictedLive; note that this will discard JSON fields in predictedLiveBytes
	// which are not in the predictedLive struct. predictedLive is thus "tainted" and we should not use it directly.
	if err = json.Unmarshal(predictedLiveBytes, &predictedLive); err == nil {

		// 1) Calls 'kubescheme.Scheme.Default(predictedLive)' and generates a patch containing the delta of that
		// call, which can then be applied to predictedLiveBytes.
		//
		// Why do we do this? Since predictedLive is "tainted" (missing extra fields), we cannot use it to populate
		// predictedLiveBytes, BUT we still need predictedLive itself in order to call the default scheme functions.
		// So, we call the default scheme functions on the "tainted" struct, to generate a patch, and then
		// apply that patch to the untainted JSON.
		patch, err := generateSchemeDefaultPatch(predictedLive)
		if err != nil {
			return nil, nil, err
		}

		// 2) Apply the default-funcs patch against the original "untainted" JSON
		// This allows us to apply the scheme default values generated above, against JSON that does not fully conform
		// to its k8s resource type (eg the JSON may contain those invalid fields that we do not wish to discard).
		predictedLiveBytes, err = strategicpatch.StrategicMergePatch(predictedLiveBytes, patch, predictedLive.DeepCopyObject())
		if err != nil {
			return nil, nil, err
		}

		// 3) Unmarshall into a map[string]interface{}, then back into byte[], to ensure the fields
		// are sorted in a consistent order (we do the same below, so that they can be
		// lexicographically compared with one another)
		var result map[string]interface{}
		err = json.Unmarshal([]byte(predictedLiveBytes), &result)
		if err != nil {
			return nil, nil, err
		}
		predictedLiveBytes, err = json.Marshal(result)
		if err != nil {
			return nil, nil, err
		}
	}

	live, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}

	// As above, unknown JSON fields in liveBytes will be discarded in the unmarshalling to 'live'.
	// However, this is much less likely since liveBytes is coming from a live k8s instance which
	// has already accepted those resources. Regardless, we still treat 'live' as tainted.
	if err = json.Unmarshal(liveBytes, live); err == nil {

		// As above, indirectly apply the schema defaults against liveBytes
		patch, err := generateSchemeDefaultPatch(live)
		if err != nil {
			return nil, nil, err
		}
		liveBytes, err = strategicpatch.StrategicMergePatch(liveBytes, patch, live.DeepCopyObject())
		if err != nil {
			return nil, nil, err
		}

		// Ensure the fields are sorted in a consistent order (as above)
		var result map[string]interface{}
		err = json.Unmarshal([]byte(liveBytes), &result)
		if err != nil {
			return nil, nil, err
		}
		liveBytes, err = json.Marshal(result)
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
	// If orig/config/live represents a registered scheme...
	if newVersionedObject != nil {
		// Apply patch while applying scheme defaults
		liveBytes, predictedLiveBytes, err = applyPatch(liveBytes, patchBytes, newVersionedObject)
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, merge patch directly as JSON
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
				annotation, ok := annotationsIf.(map[string]interface{})
				if ok && len(annotation) == 0 {
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

func GetLastAppliedConfigAnnotation(live *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if live == nil {
		return nil, nil
	}
	annotations := live.GetAnnotations()
	lastAppliedStr, ok := annotations[corev1.LastAppliedConfigAnnotation]
	if !ok {
		return nil, nil
	}
	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(lastAppliedStr), &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s in %s: %v", corev1.LastAppliedConfigAnnotation, live.GetName(), err)
	}
	return &obj, nil
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(configArray, liveArray []*unstructured.Unstructured, opts ...Option) (*DiffResultList, error) {
	numItems := len(configArray)
	if len(liveArray) != numItems {
		return nil, errors.New("left and right arrays have mismatched lengths")
	}

	diffResultList := DiffResultList{
		Diffs: make([]DiffResult, numItems),
	}
	for i := 0; i < numItems; i++ {
		config := configArray[i]
		live := liveArray[i]
		diffRes, err := Diff(config, live, opts...)
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

func Normalize(un *unstructured.Unstructured, opts ...Option) {
	if un == nil {
		return
	}
	o := applyOptions(opts)

	// creationTimestamp is sometimes set to null in the config when exported (e.g. SealedSecrets)
	// Removing the field allows a cleaner diff.
	unstructured.RemoveNestedField(un.Object, "metadata", "creationTimestamp")

	gvk := un.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "Secret" {
		NormalizeSecret(un, opts...)
	} else if gvk.Group == "rbac.authorization.k8s.io" && (gvk.Kind == "ClusterRole" || gvk.Kind == "Role") {
		normalizeRole(un, o)
	} else if gvk.Group == "" && gvk.Kind == "Endpoints" {
		normalizeEndpoint(un, o)
	}

	err := o.normalizer.Normalize(un)
	if err != nil {
		o.log.Error(err, fmt.Sprintf("Failed to normalize %s/%s/%s", un.GroupVersionKind(), un.GetNamespace(), un.GetName()))
	}
}

// NormalizeSecret mutates the supplied object and encodes stringData to data, and converts nils to
// empty strings. If the object is not a secret, or is an invalid secret, then returns the same object.
func NormalizeSecret(un *unstructured.Unstructured, opts ...Option) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return
	}
	o := applyOptions(opts)
	var secret corev1.Secret
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &secret)
	if err != nil {
		o.log.Error(err, "Failed to convert from unstructured into Secret")
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
		o.log.Error(err, "object unable to convert from secret")
		return
	}
	if secret.Data != nil {
		err = unstructured.SetNestedField(un.Object, newObj["data"], "data")
		if err != nil {
			o.log.Error(err, "failed to set secret.data")
			return
		}
	}
}

// normalizeEndpoint normalizes endpoint meaning that EndpointSubsets are sorted lexicographically
func normalizeEndpoint(un *unstructured.Unstructured, o options) {
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
		o.log.Error(err, "Failed to convert from unstructured into Endpoints")
		return
	}

	// add default protocol to subsets ports if it is empty
	for s := range ep.Subsets {
		subset := &ep.Subsets[s]
		for p := range subset.Ports {
			port := &subset.Ports[p]
			if port.Protocol == "" {
				port.Protocol = corev1.ProtocolTCP
			}
		}
	}

	endpoints.SortSubsets(ep.Subsets)

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ep)
	if err != nil {
		o.log.Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return
	}
	un.Object = newObj
}

// normalizeRole mutates the supplied Role/ClusterRole and sets rules to null if it is an empty list or an aggregated role
func normalizeRole(un *unstructured.Unstructured, o options) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "rbac.authorization.k8s.io" || (gvk.Kind != "Role" && gvk.Kind != "ClusterRole") {
		return
	}

	// Check whether the role we're checking is an aggregation role. If it is, we ignore any differences in rules.
	if o.ignoreAggregatedRoles {
		aggrIf, ok := un.Object["aggregationRule"]
		if ok {
			_, ok = aggrIf.(map[string]interface{})
			if !ok {
				o.log.Info(fmt.Sprintf("Malformed aggregationRule in resource '%s', won't modify.", un.GetName()))
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
		orig, _ = GetLastAppliedConfigAnnotation(live)
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
				// handles an edge case when secret data has nil value
				// https://github.com/argoproj/argo-cd/issues/5584
				dataValue, ok := obj.Object["data"]
				if ok {
					if dataValue == nil {
						continue
					}
				}
				var err error
				data, _, err = unstructured.NestedMap(obj.Object, "data")
				if err != nil {
					return nil, nil, fmt.Errorf("unstructured.NestedMap error: %s", err)
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
				return nil, nil, fmt.Errorf("unstructured.SetNestedField error: %s", err)
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
			return nil, nil, fmt.Errorf("error marshaling json: %s", err)
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
func remarshal(obj *unstructured.Unstructured, o options) *unstructured.Unstructured {
	obj = stripTypeInformation(obj)
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	gvk := obj.GroupVersionKind()
	item, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		// This is common. the scheme is not registered
		o.log.V(1).Info(fmt.Sprintf("Could not create new object of type %s: %v", gvk, err))
		return obj
	}
	// This will drop any omitempty fields, perform resource conversion etc...
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	// Unmarshal data into unmarshalledObj, but detect if there are any unknown fields that are not
	// found in the target GVK object.
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&unmarshalledObj); err != nil {
		// Likely a field present in obj that is not present in the GVK type, or user
		// may have specified an invalid spec in git, so return original object
		o.log.V(1).Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return obj
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		o.log.V(1).Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return obj
	}
	// Remove all default values specified by custom formatter (e.g. creationTimestamp)
	unstrBody = jsonutil.RemoveMapFields(obj.Object, unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}
}
