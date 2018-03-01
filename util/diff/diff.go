package diff

import (
	"errors"
	"fmt"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DiffResult struct {
	Diff     gojsondiff.Diff
	Modified bool
}

type DiffResultList struct {
	Diffs    []DiffResult
	Modified bool
}

// Diff performs a diff on two unstructured objects
func Diff(left, right *unstructured.Unstructured) *DiffResult {
	var leftObj, rightObj map[string]interface{}
	if left != nil {
		leftObj = left.Object
	}
	if right != nil {
		rightObj = removeMapFields(leftObj, right.Object)
	}
	gjDiff := gojsondiff.New().CompareObjects(leftObj, rightObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	return &dr
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(leftArray, rightArray []*unstructured.Unstructured) (*DiffResultList, error) {
	numItems := len(leftArray)
	if len(rightArray) != numItems {
		return nil, fmt.Errorf("left and right arrays have mismatched lengths")
	}

	diffResultList := DiffResultList{
		Diffs: make([]DiffResult, numItems),
	}
	for i := 0; i < numItems; i++ {
		left := leftArray[i]
		right := rightArray[i]
		diffRes := Diff(left, right)
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
		return removeMapFields(c, live.(map[string]interface{}))
	case []interface{}:
		return removeListFields(c, live.([]interface{}))
	default:
		return live
	}
}

func removeMapFields(config, live map[string]interface{}) map[string]interface{} {
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
	// list will be returned as is so they appear in the diff.
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
