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
	if orig != nil {
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
		liveObj = RemoveMapFields(configObj, live.Object)
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
	// remove extra fields in the live, that were not in the original object
	liveObj := RemoveMapFields(orig.Object, live.Object)
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

// https://github.com/ksonnet/ksonnet/blob/master/pkg/kubecfg/diff.go
func removeFields(config, live interface{}) interface{} {
	switch c := config.(type) {
	case map[string]interface{}:
		return RemoveMapFields(c, live.(map[string]interface{}))
	case []interface{}:
		return removeListFields(c, live.([]interface{}))
	default:
		return live
	}
}

// RemoveMapFields remove all non-existent fields in the live that don't exist in the config
func RemoveMapFields(config, live map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v1 := range config {
		v2, ok := live[k]
		if !ok {
			continue
		}
		result[k] = removeFields(v1, v2)
	}
	return result
}

func removeListFields(config, live []interface{}) []interface{} {
	// If live is longer than config, then the extra elements at the end of the
	// list will be returned as-is so they appear in the diff.
	result := make([]interface{}, 0, len(live))
	for i, v2 := range live {
		if len(config) > i {
			result = append(result, removeFields(config[i], v2))
		} else {
			result = append(result, v2)
		}
	}
	return result
}
