package diff

import (
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	jsonutil "github.com/argoproj/argo-cd/util/json"
)

type DiffResult struct {
	Diff     gojsondiff.Diff
	Modified bool
}

type DiffResultList struct {
	Diffs    []DiffResult
	Modified bool
}

// Diff performs a diff on two unstructured objects. If the live object happens to have a
// "kubectl.kubernetes.io/last-applied-configuration", then perform a three way diff.
func Diff(config, live *unstructured.Unstructured) *DiffResult {
	if config != nil {
		config = stripTypeInformation(config)
		encodeSecretStringData(config)
	}
	if live != nil {
		live = stripTypeInformation(live)
	}
	orig := getLastAppliedConfigAnnotation(live)
	if orig != nil && config != nil {
		dr, err := ThreeWayDiff(orig, config, live)
		if err == nil {
			return dr
		}
		log.Debugf("three-way diff calculation failed: %v. Falling back to two-way diff", err)
	}
	return TwoWayDiff(config, live)
}

// TwoWayDiff performs a normal two-way diff between two unstructured objects. Ignores extra fields
// in the live object.
// Inputs are assumed to be stripped of type information
func TwoWayDiff(config, live *unstructured.Unstructured) *DiffResult {
	var configObj, liveObj map[string]interface{}
	if config != nil {
		config = removeNamespaceAnnotation(config)
		configObj = config.Object
	}
	if live != nil {
		liveObj = jsonutil.RemoveMapFields(configObj, live.Object)
	}
	gjDiff := gojsondiff.New().CompareObjects(liveObj, configObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	return &dr
}

// ThreeWayDiff performs a diff with the understanding of how to incorporate the
// last-applied-configuration annotation in the diff.
// Inputs are assumed to be stripped of type information
func ThreeWayDiff(orig, config, live *unstructured.Unstructured) (*DiffResult, error) {
	orig = removeNamespaceAnnotation(orig)
	config = removeNamespaceAnnotation(config)
	// Remove defaulted fields from the live object.
	// This subtracts any extra fields in the live object which are not present in last-applied-configuration.
	// This is needed to perform a fair comparison when we send the objects to gojsondiff
	live = &unstructured.Unstructured{Object: jsonutil.RemoveMapFields(orig.Object, live.Object)}

	// 1. calculate a 3-way merge patch
	patchBytes, err := threeWayMergePatch(orig, config, live)
	if err != nil {
		return nil, err
	}

	// 2. apply the patch against the live object
	liveBytes, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}
	versionedObject, err := scheme.Scheme.New(orig.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	patchedLiveBytes, err := strategicpatch.StrategicMergePatch(liveBytes, patchBytes, versionedObject)
	if err != nil {
		return nil, err
	}
	var patchedLive unstructured.Unstructured
	err = json.Unmarshal(patchedLiveBytes, &patchedLive)
	if err != nil {
		return nil, err
	}

	// 3. diff the live object vs. the patched live object
	gjDiff := gojsondiff.New().CompareObjects(live.Object, patchedLive.Object)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
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

func threeWayMergePatch(orig, config, live *unstructured.Unstructured) ([]byte, error) {
	origBytes, err := json.Marshal(orig.Object)
	if err != nil {
		return nil, err
	}
	configBytes, err := json.Marshal(config.Object)
	if err != nil {
		return nil, err
	}
	liveBytes, err := json.Marshal(live.Object)
	if err != nil {
		return nil, err
	}
	gvk := orig.GroupVersionKind()
	versionedObject, err := scheme.Scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateThreeWayMergePatch(origBytes, configBytes, liveBytes, lookupPatchMeta, true)
}

func getLastAppliedConfigAnnotation(live *unstructured.Unstructured) *unstructured.Unstructured {
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
		log.Warnf("Failed to unmarshal %s in %s", core.LastAppliedConfigAnnotation, live.GetName())
		return nil
	}
	return &obj
}

// MatchObjectLists takes two possibly disjoint lists of Unstructured objects, and returns two new
// lists of equal lengths, filled out with nils from missing objects in the opposite list.
// These lists can then be passed into DiffArray for comparison
func MatchObjectLists(leftObjs, rightObjs []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured) {
	newLeftObjs := make([]*unstructured.Unstructured, 0)
	newRightObjs := make([]*unstructured.Unstructured, 0)

	for _, left := range leftObjs {
		if left == nil {
			continue
		}
		newLeftObjs = append(newLeftObjs, left)
		right := objByKindName(rightObjs, left.GetKind(), left.GetName())
		newRightObjs = append(newRightObjs, right)
	}

	for _, right := range rightObjs {
		if right == nil {
			continue
		}
		left := objByKindName(leftObjs, right.GetKind(), right.GetName())
		if left != nil {
			// object exists in both list. this object was already appended to both lists in the
			// first for/loop
			continue
		}
		// if we get here, we found a right which doesn't exist in the left object list.
		// append a nil to the left object list
		newLeftObjs = append(newLeftObjs, nil)
		newRightObjs = append(newRightObjs, right)

	}
	return newLeftObjs, newRightObjs
}

func objByKindName(objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		if obj.GetKind() == kind && obj.GetName() == name {
			return obj
		}
	}
	return nil
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(configArray, liveArray []*unstructured.Unstructured) (*DiffResultList, error) {
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
		diffRes := Diff(config, live)
		diffResultList.Diffs[i] = *diffRes
		if diffRes.Modified {
			diffResultList.Modified = true
		}
	}
	return &diffResultList, nil
}

// ASCIIFormat returns the ASCII format of the diff
func (d *DiffResult) ASCIIFormat(left *unstructured.Unstructured, formatOpts formatter.AsciiFormatterConfig) (string, error) {
	if !d.Diff.Modified() {
		return "", nil
	}
	if left == nil {
		return "", errors.New("Supplied nil left object")
	}
	asciiFmt := formatter.NewAsciiFormatter(left.Object, formatOpts)
	return asciiFmt.Format(d.Diff)
}

// encodeSecretStringData mutates the supplied object and encodes stringData to data. If the object
// is not a secret, or is an invalid secret, then returns the same object
func encodeSecretStringData(un *unstructured.Unstructured) {
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
		log.Warnf("object unable to convert to secret: %v", err)
		return
	}
	if len(secret.StringData) == 0 {
		return
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	for k, v := range secret.StringData {
		secret.Data[k] = []byte(v)
	}
	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&secret)
	if err != nil {
		log.Warnf("object unable to convert from secret: %v", err)
		return
	}
	err = unstructured.SetNestedMap(un.Object, newObj["data"].(map[string]interface{}), "data")
	if err != nil {
		log.Warnf("failed to set secret.data: %v", err)
		return
	}
	delete(un.Object, "stringData")
}

// UnmarshalDiffString unmarshals diff string into a DiffResult struct
func UnmarshalDiffString(diffStr string) (*DiffResult, error) {
	diffUnmarshaller := gojsondiff.NewUnmarshaller()
	diffResult := &DiffResult{}
	diff, err := diffUnmarshaller.UnmarshalString(diffStr)
	if err != nil {
		return nil, err
	}
	diffResult.Diff = diff
	if diffStr != "" {
		diffResult.Modified = true
	}
	return diffResult, nil
}

// JSONFormat returns the diff as a JSON string
func (d *DiffResult) JSONFormat() (string, error) {
	if !d.Diff.Modified() {
		return "", nil
	}
	jsonFmt := formatter.NewDeltaFormatter()
	return jsonFmt.Format(d.Diff)
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
