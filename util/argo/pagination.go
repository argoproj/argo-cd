package argo

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
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

// GetProjectsFromApplicationQuery gets the project names from a query. If the legacy "project" field was specified, use
// that. Otherwise, use the newer "projects" field.
func GetProjectsFromApplicationQuery(q application.ApplicationQuery) []string {
	if q.Project != nil {
		return q.Project
	}
	return q.Projects
}

func getReposFromApplicationQuery(q application.ApplicationQuery) []string {
	if q.Repo != nil {
		return []string{*q.Repo}
	}
	if q.Options != nil && q.Options.Repos != nil {
		return q.Options.Repos
	}
	return []string{}
}

func BuildFilter(q application.ApplicationQuery) Filter {
	chainFilter := NewChainFilter()

	if q.Name != nil {
		f := NewStringPropertyFilter([]string{q.GetName()}, func(app *v1alpha1.Application) string { return app.Name })
		chainFilter.AddFilter(f)
	}

	if GetProjectsFromApplicationQuery(q) != nil {
		f := NewStringPropertyFilter(GetProjectsFromApplicationQuery(q), func(app *v1alpha1.Application) string { return app.Spec.Project })
		chainFilter.AddFilter(f)
	}

	if len(getReposFromApplicationQuery(q)) > 0 {
		f := NewStringPropertyFilter(getReposFromApplicationQuery(q), func(app *v1alpha1.Application) string { return app.Spec.Source.RepoURL })
		chainFilter.AddFilter(f)
	}

	if q.Options != nil {
		o := q.Options
		if o.MinName != nil {
			chainFilter.AddFilter(NewMinNameFilter(o.GetMinName()))
		}

		if o.MaxName != nil {
			chainFilter.AddFilter(NewMaxNameFilter(o.GetMaxName()))
		}

		if len(o.GetClusters()) > 0 {
			chainFilter.AddFilter(NewClustersFilter(o.GetClusters()))
		}

		if len(o.GetNamespaces()) > 0 {
			f := NewStringPropertyFilter(o.GetNamespaces(), func(app *v1alpha1.Application) string { return app.Spec.Destination.Namespace })
			chainFilter.AddFilter(f)
		}

		if o.AutoSyncEnabled != nil {
			f := NewBoolPropertyFilter(*o.AutoSyncEnabled, func(app *v1alpha1.Application) bool {
				return app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil
			})
			chainFilter.AddFilter(f)
		}

		if len(o.GetSyncStatuses()) > 0 {
			f := NewStringPropertyFilter(o.GetSyncStatuses(), func(app *v1alpha1.Application) string { return string(app.Status.Sync.Status) })
			chainFilter.AddFilter(f)
		}

		if len(o.GetHealthStatuses()) > 0 {
			f := NewStringPropertyFilter(o.GetHealthStatuses(), func(app *v1alpha1.Application) string { return string(app.Status.Health.Status) })
			chainFilter.AddFilter(f)
		}

		if o.Search != nil {
			chainFilter.AddFilter(NewSearchFilter(o.GetSearch()))
		}
	}

	return chainFilter
}

func Paginate(apps []*v1alpha1.Application, q application.ApplicationQuery) ([]*v1alpha1.Application, *application.ApplicationListStats, error) {
	stats := getAppsStats(apps)
	filter := BuildFilter(q)
	filteredApps := FilterByFiltersP(apps, filter)

	less, err := getApplicationLessFunc(q.Options.GetSortBy())
	if err != nil {
		return nil, nil, fmt.Errorf("error sorting applications: %w", err)
	}
	sort.SliceStable(filteredApps, func(i, j int) bool {
		return less(filteredApps[i], filteredApps[j])
	})

	if q.Options != nil {
		if q.Options.Offset != nil && q.Options.Limit != nil {
			offset, limit := q.Options.GetOffset(), q.Options.GetLimit()
			if offset < 0 || limit < 0 {
				return nil, nil, fmt.Errorf("offset %d and limit %d must be a non-negative integer", offset, limit)
			}
			if offset >= int64(len(filteredApps)) || limit == 0 {
				filteredApps = make([]*v1alpha1.Application, 0)
			} else {
				if offset+limit >= int64(len(filteredApps)) {
					filteredApps = filteredApps[offset:]
				} else {
					filteredApps = filteredApps[offset : offset+limit]
				}
			}
		}
	}
	updateStatsWithFilteredApps(stats, filteredApps)
	return filteredApps, stats, nil
}

func getApplicationLessFunc(sortBy application.ApplicationSortBy) (func(x, y *v1alpha1.Application) bool, error) {
	if sortBy == application.ApplicationSortBy_ASB_UNSPECIFIED || sortBy == application.ApplicationSortBy_ASB_NAME {
		return func(x, y *v1alpha1.Application) bool {
			return x.Name < y.Name
		}, nil
	}
	// sort in descending order
	if sortBy == application.ApplicationSortBy_ASB_CREATED_AT {
		return func(y, x *v1alpha1.Application) bool {
			if x.CreationTimestamp.Equal(&y.CreationTimestamp) {
				// If creation timestamps are equal, sort by name
				return y.Name < x.Name
			}
			return x.CreationTimestamp.Before(&y.CreationTimestamp)
		}, nil
	}
	// sort in descending order
	if sortBy == application.ApplicationSortBy_ASB_SYNCHRONIZED {
		// If x.FinishedAt was assigned but y not, we think x is before(less) than y
		return func(y, x *v1alpha1.Application) bool {
			if x.Status.OperationState != nil {
				if y.Status.OperationState != nil {
					if x.Status.OperationState.FinishedAt.Equal(y.Status.OperationState.FinishedAt) {
						// If finished timestamps are equal, sort by name
						return y.Name < x.Name
					}
					return x.Status.OperationState.FinishedAt.Before(y.Status.OperationState.FinishedAt)
				}
				return true
			}
			if y.Status.OperationState != nil {
				return false
			}
			// Sort by name if both were nil
			return y.Name < x.Name
		}, nil
	}
	return nil, fmt.Errorf("invalid sort by %s", sortBy)
}

// updateStatsWithFilteredApps returns a summary of applications after filtering.
// It counts the total number of applications, grouped by health status and sync status
// so that the UI can do pagination.
func updateStatsWithFilteredApps(stats *application.ApplicationListStats, apps []*v1alpha1.Application) {
	total := int64(len(apps))
	stats.Total = &total
	stats.AutoSyncEnabledCount = new(int64)
	stats.TotalByHealthStatus = make(map[string]int64)
	stats.TotalBySyncStatus = make(map[string]int64)
	for _, app := range apps {
		stats.TotalByHealthStatus[string(app.Status.Health.Status)]++
		stats.TotalBySyncStatus[string(app.Status.Sync.Status)]++
		if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil {
			*stats.AutoSyncEnabledCount++
		}
	}
}

// getAppsStats returns a summary of all applications before pagination and filtering.
// It collects unique destinations, namespaces, and labels across all applications.
func getAppsStats(apps []*v1alpha1.Application) *application.ApplicationListStats {
	stats := &application.ApplicationListStats{}
	destinations := map[v1alpha1.ApplicationDestination]bool{}
	namespaces := map[string]bool{}
	labels := map[string]map[string]bool{}
	for _, app := range apps {
		if _, ok := destinations[app.Spec.Destination]; !ok {
			destinations[app.Spec.Destination] = true
		}
		if _, ok := namespaces[app.Spec.Destination.Namespace]; !ok {
			namespaces[app.Spec.Destination.Namespace] = true
		}
		for key, value := range app.Labels {
			if valueMap, ok := labels[key]; !ok {
				labels[key] = map[string]bool{value: true}
			} else {
				valueMap[value] = true
			}
		}
	}
	for key := range destinations {
		stats.Destinations = append(stats.Destinations, key.DeepCopy())
	}
	stats.Namespaces = slices.Collect(maps.Keys(namespaces))
	for key, valueMap := range labels {
		stats.Labels = append(stats.Labels, &application.ApplicationLabelStats{
			Key:    &key,
			Values: slices.Collect(maps.Keys(valueMap)),
		})
	}
	return stats
}
