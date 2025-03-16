package argo

import (
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type Filter interface {
	IsValid(*v1alpha1.Application) (ok bool)
}

type ChainFilter struct {
	filters []Filter
}

func NewChainFilter() *ChainFilter {
	return &ChainFilter{
		filters: make([]Filter, 0),
	}
}

func (c *ChainFilter) AddFilter(filter Filter) {
	c.filters = append(c.filters, filter)
}

func (c *ChainFilter) IsValid(app *v1alpha1.Application) bool {
	for _, filter := range c.filters {
		if !filter.IsValid(app) {
			return false
		}
	}
	return true
}

type MinNameFilter struct {
	minName string
}

func NewMinNameFilter(minName string) *MinNameFilter {
	return &MinNameFilter{
		minName: minName,
	}
}

func (f *MinNameFilter) IsValid(app *v1alpha1.Application) bool {
	return strings.HasPrefix(app.Name, f.minName)
}

func NewMaxNameFilter(maxName string) *MaxNameFilter {
	return &MaxNameFilter{
		maxName: maxName,
	}
}

type MaxNameFilter struct {
	maxName string
}

func (f *MaxNameFilter) IsValid(app *v1alpha1.Application) bool {
	return strings.HasSuffix(app.Name, f.maxName)
}

type ClustersFilter struct {
	clusters map[string]struct{}
}

func NewClustersFilter(clusters []string) *ClustersFilter {
	clustersMap := make(map[string]struct{})
	for _, cluster := range clusters {
		clustersMap[cluster] = struct{}{}
	}
	return &ClustersFilter{
		clusters: clustersMap,
	}
}

func (f *ClustersFilter) IsValid(app *v1alpha1.Application) bool {
	if _, ok := f.clusters[app.Spec.Destination.Name]; !ok {
		if _, ok := f.clusters[app.Spec.Destination.Server]; !ok {
			return false
		}
	}
	return true
}

type StringPropertyFilter struct {
	validValues  map[string]struct{}
	propertyFunc func(app *v1alpha1.Application) string
}

func NewStringPropertyFilter(validValues []string, propertyFunc func(app *v1alpha1.Application) string) *StringPropertyFilter {
	validValuesMap := make(map[string]struct{})
	for _, value := range validValues {
		validValuesMap[value] = struct{}{}
	}
	return &StringPropertyFilter{
		validValues:  validValuesMap,
		propertyFunc: propertyFunc,
	}
}

func (f *StringPropertyFilter) IsValid(app *v1alpha1.Application) bool {
	value := f.propertyFunc(app)
	if _, ok := f.validValues[value]; !ok {
		return false
	}
	return true
}

// BoolPropertyFilter returns true if the property of application is enabled
type BoolPropertyFilter struct {
	validValue bool
	propertyFn func(app *v1alpha1.Application) bool
}

func NewBoolPropertyFilter(validValue bool, propertyFunc func(app *v1alpha1.Application) bool) *BoolPropertyFilter {
	return &BoolPropertyFilter{
		validValue: validValue,
		propertyFn: propertyFunc,
	}
}

func (f *BoolPropertyFilter) IsValid(app *v1alpha1.Application) bool {
	return f.propertyFn(app) == f.validValue
}

type SearchFilter struct {
	search string
}

func NewSearchFilter(search string) *SearchFilter {
	return &SearchFilter{
		search: search,
	}
}

func (f *SearchFilter) IsValid(app *v1alpha1.Application) bool {
	return strings.Contains(app.Name, f.search)
}

func FilterByFiltersP(apps []*v1alpha1.Application, f Filter) []*v1alpha1.Application {
	var filteredApps []*v1alpha1.Application
	for _, app := range apps {
		if f.IsValid(app) {
			filteredApps = append(filteredApps, app)
		}
	}
	return filteredApps
}
