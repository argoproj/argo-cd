package diff

import (
	"fmt"
	"strings"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DiffResult struct {
	Diff          gojsondiff.Diff `json:"-"`
	Modified      bool            `json:"modified"`
	AdditionsOnly *bool           `json:"additionsOnly,omitempty"`
	ASCIIDiff     string          `json:"asciiDiff,omitempty"`
	DeltaDiff     string          `json:"deltaDiff,omitempty"`
}

// Diff performs a diff on two unstructured objects
func Diff(left, right *unstructured.Unstructured) *DiffResult {
	var leftObj, rightObj map[string]interface{}
	if left != nil {
		leftObj = left.Object
	}
	if right != nil {
		rightObj = right.Object
	}
	gjDiff := gojsondiff.New().CompareObjects(leftObj, rightObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	dr.renderOutput(leftObj)
	return &dr
}

type DiffResultList struct {
	Diffs         []DiffResult `json:"diffs,omitempty"`
	Modified      bool         `json:"modified"`
	AdditionsOnly *bool        `json:"additionsOnly,omitempty"`
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
			if !*diffRes.AdditionsOnly {
				diffResultList.AdditionsOnly = diffRes.AdditionsOnly
			}
		}
	}
	if diffResultList.Modified && diffResultList.AdditionsOnly == nil {
		t := true
		diffResultList.AdditionsOnly = &t
	}
	return &diffResultList, nil
}

// renderOutput is a helper to render the output and check if the modifications are only additions
func (d *DiffResult) renderOutput(left interface{}) {
	if !d.Diff.Modified() {
		return
	}
	asciiFmt := formatter.NewAsciiFormatter(left, formatter.AsciiFormatterConfig{})
	var err error
	d.ASCIIDiff, err = asciiFmt.Format(d.Diff)
	if err != nil {
		panic(err)
	}
	deltaFmt := formatter.NewDeltaFormatter()
	d.DeltaDiff, err = deltaFmt.Format(d.Diff)
	if err != nil {
		panic(err)
	}

	for _, line := range strings.Split(d.ASCIIDiff, "\n") {
		if len(line) == 0 {
			continue
		}
		switch string(line[0]) {
		case formatter.AsciiAdded, formatter.AsciiSame:
		default:
			f := false
			d.AdditionsOnly = &f
			return
		}
	}
	t := true
	d.AdditionsOnly = &t
}
