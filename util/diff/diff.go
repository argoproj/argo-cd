package diff

import (
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	orig := getLastAppliedConfigAnnotation(live)
	if orig != nil && config != nil {
		return ThreeWayDiff(orig, config, live)
	}
	return TwoWayDiff(config, live)
}

// TwoWayDiff performs a normal two-way diff between two unstructured objects. Ignores extra fields
// in the live object.
func TwoWayDiff(config, live *unstructured.Unstructured) *DiffResult {
	var configObj, liveObj map[string]interface{}
	if config != nil {
		configObj = config.Object
	}
	if live != nil {
		liveObj = jsonutil.RemoveMapFields(configObj, live.Object)
	}
	gjDiff := gojsondiff.New().CompareObjects(configObj, liveObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	return &dr
}

// ThreeWayDiff performs a diff with the understanding of how to incorporate the
// last-applied-configuration annotation in the diff.
func ThreeWayDiff(orig, config, live *unstructured.Unstructured) *DiffResult {
	orig = removeNamespaceAnnotation(orig)
	// remove extra fields in the live, that were not in the original object
	liveObj := jsonutil.RemoveMapFields(orig.Object, live.Object)
	// now we have a pruned live object
	gjDiff := gojsondiff.New().CompareObjects(config.Object, liveObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	// Theoretically, we should be able to return the diff result a this point. Just to be safe,
	// calculate a kubernetes 3-way merge patch to see if kubernetes will also agree with what we
	// just calculated.
	patch, err := threeWayMergePatch(orig, config, live)
	if err != nil {
		log.Warnf("Failed to calculate three way merge patch: %v", err)
		return &dr
	}
	patchStr := string(patch)
	modified := bool(patchStr != "{}")
	if dr.Modified != modified {
		// We theoretically should not get here. If we do, it is a issue with our diff calculation
		// We should honor what kubernetes thinks. If we *do* get here, it means what we will be
		// reporting OutOfSync, but do not have a good way to visualize the diff to the user.
		log.Warnf("Disagreement in three way diff calculation: %s", patchStr)
		dr.Modified = modified
	}
	return &dr
}

func removeNamespaceAnnotation(orig *unstructured.Unstructured) *unstructured.Unstructured {
	orig = orig.DeepCopy()
	// remove the namespace an annotation from the
	if metadataIf, ok := orig.Object["metadata"]; ok {
		metadata := metadataIf.(map[string]interface{})
		delete(metadata, "namespace")
		if annotationsIf, ok := metadata["annotations"]; ok {
			annotation := annotationsIf.(map[string]interface{})
			if len(annotation) == 0 {
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
	lastAppliedStr, ok := annots[v1.LastAppliedConfigAnnotation]
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
