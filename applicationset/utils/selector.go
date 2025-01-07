package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
)

var (
	unaryOperators = []string{
		string(selection.Exists), string(selection.DoesNotExist),
	}
	binaryOperators = []string{
		string(selection.In), string(selection.NotIn),
		string(selection.Equals), string(selection.DoubleEquals), string(selection.NotEquals),
		string(selection.GreaterThan), string(selection.LessThan),
	}
	validRequirementOperators = append(binaryOperators, unaryOperators...)
)

// Selector represents a label selector.
type Selector interface {
	// Matches returns true if this selector matches the given set of labels.
	Matches(labels.Labels) bool

	// Add adds requirements to the Selector
	Add(r ...Requirement) Selector
}

type internalSelector []Requirement

// ByKey sorts requirements by key to obtain deterministic parser
type ByKey []Requirement

func (a ByKey) Len() int { return len(a) }

func (a ByKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (a ByKey) Less(i, j int) bool { return a[i].key < a[j].key }

// Matches for a internalSelector returns true if all
// its Requirements match the input Labels. If any
// Requirement does not match, false is returned.
func (s internalSelector) Matches(l labels.Labels) bool {
	for ix := range s {
		if matches := s[ix].Matches(l); !matches {
			return false
		}
	}
	return true
}

// Add adds requirements to the selector. It copies the current selector returning a new one
func (s internalSelector) Add(reqs ...Requirement) Selector {
	ret := make(internalSelector, 0, len(s)+len(reqs))
	ret = append(ret, s...)
	ret = append(ret, reqs...)
	sort.Sort(ByKey(ret))
	return ret
}

type nothingSelector struct{}

func (n nothingSelector) Matches(l labels.Labels) bool {
	return false
}

func (n nothingSelector) Add(r ...Requirement) Selector {
	return n
}

// Nothing returns a selector that matches no labels
func nothing() Selector {
	return nothingSelector{}
}

// Everything returns a selector that matches all labels.
func everything() Selector {
	return internalSelector{}
}

// LabelSelectorAsSelector converts the LabelSelector api type into a struct that implements
// labels.Selector
// Note: This function should be kept in sync with the selector methods in pkg/labels/selector.go
func LabelSelectorAsSelector(ps *v1.LabelSelector) (Selector, error) {
	if ps == nil {
		return nothing(), nil
	}
	if len(ps.MatchLabels)+len(ps.MatchExpressions) == 0 {
		return everything(), nil
	}
	requirements := make([]Requirement, 0, len(ps.MatchLabels)+len(ps.MatchExpressions))
	for k, v := range ps.MatchLabels {
		r, err := newRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *r)
	}
	for _, expr := range ps.MatchExpressions {
		var op selection.Operator
		switch expr.Operator {
		case v1.LabelSelectorOpIn:
			op = selection.In
		case v1.LabelSelectorOpNotIn:
			op = selection.NotIn
		case v1.LabelSelectorOpExists:
			op = selection.Exists
		case v1.LabelSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		default:
			return nil, fmt.Errorf("%q is not a valid pod selector operator", expr.Operator)
		}
		r, err := newRequirement(expr.Key, op, append([]string(nil), expr.Values...))
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *r)
	}
	selector := newSelector()
	selector = selector.Add(requirements...)
	return selector, nil
}

// NewSelector returns a nil selector
func newSelector() Selector {
	return internalSelector(nil)
}

func validateLabelKey(k string, path *field.Path) *field.Error {
	if errs := validation.IsQualifiedName(k); len(errs) != 0 {
		return field.Invalid(path, k, strings.Join(errs, "; "))
	}
	return nil
}

// NewRequirement is the constructor for a Requirement.
// If any of these rules is violated, an error is returned:
// (1) The operator can only be In, NotIn, Equals, DoubleEquals, Gt, Lt, NotEquals, Exists, or DoesNotExist.
// (2) If the operator is In or NotIn, the values set must be non-empty.
// (3) If the operator is Equals, DoubleEquals, or NotEquals, the values set must contain one value.
// (4) If the operator is Exists or DoesNotExist, the value set must be empty.
// (5) If the operator is Gt or Lt, the values set must contain only one value, which will be interpreted as an integer.
// (6) The key is invalid due to its length, or sequence
//
//	of characters. See validateLabelKey for more details.
//
// The empty string is a valid value in the input values set.
// Returned error, if not nil, is guaranteed to be an aggregated field.ErrorList
func newRequirement(key string, op selection.Operator, vals []string, opts ...field.PathOption) (*Requirement, error) {
	var allErrs field.ErrorList
	path := field.ToPath(opts...)
	if err := validateLabelKey(key, path.Child("key")); err != nil {
		allErrs = append(allErrs, err)
	}

	valuePath := path.Child("values")
	switch op {
	case selection.In, selection.NotIn:
		if len(vals) == 0 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "for 'in', 'notin' operators, values set can't be empty"))
		}
	case selection.Equals, selection.DoubleEquals, selection.NotEquals:
		if len(vals) != 1 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "exact-match compatibility requires one single value"))
		}
	case selection.Exists, selection.DoesNotExist:
		if len(vals) != 0 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "values set must be empty for exists and does not exist"))
		}
	case selection.GreaterThan, selection.LessThan:
		if len(vals) != 1 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "for 'Gt', 'Lt' operators, exactly one value is required"))
		}
		for i := range vals {
			if _, err := strconv.ParseInt(vals[i], 10, 64); err != nil {
				allErrs = append(allErrs, field.Invalid(valuePath.Index(i), vals[i], "for 'Gt', 'Lt' operators, the value must be an integer"))
			}
		}
	default:
		allErrs = append(allErrs, field.NotSupported(path.Child("operator"), op, validRequirementOperators))
	}

	return &Requirement{key: key, operator: op, strValues: vals}, allErrs.ToAggregate()
}

// Requirement contains values, a key, and an operator that relates the key and values.
// The zero value of Requirement is invalid.
// Requirement implements both set based match and exact match
// Requirement should be initialized via NewRequirement constructor for creating a valid Requirement.
// +k8s:deepcopy-gen=true
type Requirement struct {
	key      string
	operator selection.Operator
	// In the majority of cases we have at most one value here.
	// It is generally faster to operate on a single-element slice
	// than on a single-element map, so we have a slice here.
	strValues []string
}

func (r *Requirement) hasValue(value string) bool {
	for i := range r.strValues {
		if r.strValues[i] == value {
			return true
		}
	}
	return false
}

func (r *Requirement) Matches(ls labels.Labels) bool {
	switch r.operator {
	case selection.In, selection.Equals, selection.DoubleEquals:
		if !ls.Has(r.key) {
			return false
		}
		return r.hasValue(ls.Get(r.key))
	case selection.NotIn, selection.NotEquals:
		if !ls.Has(r.key) {
			return true
		}
		return !r.hasValue(ls.Get(r.key))
	case selection.Exists:
		return ls.Has(r.key)
	case selection.DoesNotExist:
		return !ls.Has(r.key)
	case selection.GreaterThan, selection.LessThan:
		if !ls.Has(r.key) {
			return false
		}
		lsValue, err := strconv.ParseInt(ls.Get(r.key), 10, 64)
		if err != nil {
			klog.V(10).Infof("ParseInt failed for value %+v in label %+v, %+v", ls.Get(r.key), ls, err)
			return false
		}

		// There should be only one strValue in r.strValues, and can be converted to an integer.
		if len(r.strValues) != 1 {
			klog.V(10).Infof("Invalid values count %+v of requirement %#v, for 'Gt', 'Lt' operators, exactly one value is required", len(r.strValues), r)
			return false
		}

		var rValue int64
		for i := range r.strValues {
			rValue, err = strconv.ParseInt(r.strValues[i], 10, 64)
			if err != nil {
				klog.V(10).Infof("ParseInt failed for value %+v in requirement %#v, for 'Gt', 'Lt' operators, the value must be an integer", r.strValues[i], r)
				return false
			}
		}
		return (r.operator == selection.GreaterThan && lsValue > rValue) || (r.operator == selection.LessThan && lsValue < rValue)
	default:
		return false
	}
}
