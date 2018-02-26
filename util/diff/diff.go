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
	Output        string          `json:"output,omitempty"`
}

// Diff performs a diff on two unstructured objects
func Diff(left, right *unstructured.Unstructured) *DiffResult {
	gjDiff := gojsondiff.New().CompareObjects(left.Object, right.Object)
	out, additions := renderOutput(left.Object, gjDiff)
	return &DiffResult{
		Diff:          gjDiff,
		Output:        out,
		AdditionsOnly: additions,
		Modified:      gjDiff.Modified(),
	}
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
func renderOutput(left interface{}, diff gojsondiff.Diff) (string, *bool) {
	if !diff.Modified() {
		return "", nil
	}
	diffFmt := formatter.NewAsciiFormatter(left, formatter.AsciiFormatterConfig{})
	out, err := diffFmt.Format(diff)
	if err != nil {
		panic(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) == 0 {
			continue
		}
		switch string(line[0]) {
		case formatter.AsciiAdded, formatter.AsciiSame:
		default:
			f := false
			return out, &f
		}
	}
	t := true
	return out, &t
}
