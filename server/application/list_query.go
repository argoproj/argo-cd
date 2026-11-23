package application

import (
	"regexp"
	"sort"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
)

const (
	sortCreatedAt      = "createdAt"
	sortSynchronizedAt = "synchronizedAt"
)

func compileSearch(q *application.ApplicationQuery) (search string, searchRe *regexp.Regexp, ok bool) {
	search = q.GetSearch()
	if q.GetSearchRegex() && search != "" {
		re, err := regexp.Compile(search)
		if err != nil {
			return "", nil, false
		}
		return "", re, true
	}
	return search, nil, true
}

func filterApplicationsByQuery(apps []*v1alpha1.Application, q *application.ApplicationQuery) []*v1alpha1.Application {
	search, searchRe, ok := compileSearch(q)
	if !ok {
		return nil
	}
	var names map[string]bool
	if len(q.GetNames()) > 0 {
		names = argo.NewNameFilter(q.GetNames())
	}
	filtered := make([]*v1alpha1.Application, 0, len(apps))
	for _, app := range apps {
		if matchesApplicationQuery(app, q, search, searchRe, names) {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

// matchesApplicationQueryQ compiles search once and matches a single application (Watch path).
func matchesApplicationQueryQ(app *v1alpha1.Application, q *application.ApplicationQuery) bool {
	search, searchRe, ok := compileSearch(q)
	if !ok {
		return false
	}
	var names map[string]bool
	if len(q.GetNames()) > 0 {
		names = argo.NewNameFilter(q.GetNames())
	}
	return matchesApplicationQuery(app, q, search, searchRe, names)
}

func matchesApplicationQuery(app *v1alpha1.Application, q *application.ApplicationQuery, search string, searchRe *regexp.Regexp, names map[string]bool) bool {
	if names != nil {
		if !argo.MatchesNameFilter(names, app.Namespace, app.Name) {
			return false
		}
	}
	if !argo.MatchesRepos(app, q.GetRepos()) {
		return false
	}
	if !argo.MatchesClusters(app, q.GetClusters()) {
		return false
	}
	if !argo.MatchesNamespaces(app, q.GetNamespaces()) {
		return false
	}
	if !argo.MatchesLabels(app, q.GetLabels()) {
		return false
	}
	if !argo.MatchesAutoSync(app, q.GetAutoSyncs()) {
		return false
	}
	if !argo.MatchesSyncStatuses(app, q.GetSyncStatuses()) {
		return false
	}
	if !argo.MatchesHealthStatuses(app, q.GetHealthStatuses()) {
		return false
	}
	if !argo.MatchesSearchCompiled(app, search, searchRe) {
		return false
	}
	if q.MinName != nil && app.Name < *q.MinName {
		return false
	}
	if q.MaxName != nil && app.Name > *q.MaxName {
		return false
	}
	return true
}

func lessByNamespaceName(a, b *v1alpha1.Application) bool {
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	return a.Name < b.Name
}

func sortApplications(apps []*v1alpha1.Application, sortBy string) {
	switch sortBy {
	case sortCreatedAt:
		sort.Slice(apps, func(i, j int) bool {
			if !apps[i].CreationTimestamp.Equal(&apps[j].CreationTimestamp) {
				return apps[i].CreationTimestamp.After(apps[j].CreationTimestamp.Time)
			}
			return lessByNamespaceName(apps[i], apps[j])
		})
	case sortSynchronizedAt:
		sort.Slice(apps, func(i, j int) bool {
			timeI, timeJ := synchronizedAt(apps[i]), synchronizedAt(apps[j])
			if timeI.IsZero() && !timeJ.IsZero() {
				return false
			}
			if !timeI.IsZero() && timeJ.IsZero() {
				return true
			}
			if timeI.IsZero() && timeJ.IsZero() {
				return lessByNamespaceName(apps[i], apps[j])
			}
			if !timeI.Equal(timeJ) {
				return timeI.After(timeJ)
			}
			return lessByNamespaceName(apps[i], apps[j])
		})
	default:
		sort.Slice(apps, func(i, j int) bool {
			return lessByNamespaceName(apps[i], apps[j])
		})
	}
}

func synchronizedAt(app *v1alpha1.Application) time.Time {
	if app.Status.OperationState != nil && app.Status.OperationState.FinishedAt != nil {
		return app.Status.OperationState.FinishedAt.Time
	}
	return time.Time{}
}

func paginateApplications(apps []*v1alpha1.Application, q *application.ApplicationQuery) []*v1alpha1.Application {
	if q.Limit == nil || *q.Limit <= 0 {
		return apps
	}
	offset := int64(0)
	if q.Offset != nil && *q.Offset > 0 {
		offset = *q.Offset
	}
	n := int64(len(apps))
	start := min(offset, n)
	if start >= n {
		return apps[start:start]
	}
	remaining := n - start
	limit := min(*q.Limit, remaining)
	return apps[start : start+limit]
}

// buildApplicationListStats computes interdependent facet counts over the RBAC-permitted set.
// For sync/health/autosync facets, all query filters except that facet dimension are applied.
func buildApplicationListStats(apps []*v1alpha1.Application, q *application.ApplicationQuery) v1alpha1.ApplicationListStats {
	stats := v1alpha1.ApplicationListStats{
		TotalBySyncStatus:   make(map[v1alpha1.SyncStatusCode]int64),
		TotalByHealthStatus: make(map[health.HealthStatusCode]int64),
		AutoSyncCount:       make(map[string]int64),
	}

	qWithoutSync := *q
	qWithoutSync.SyncStatuses = nil
	for _, app := range filterApplicationsByQuery(apps, &qWithoutSync) {
		stats.TotalBySyncStatus[app.Status.Sync.Status]++
	}

	qWithoutHealth := *q
	qWithoutHealth.HealthStatuses = nil
	for _, app := range filterApplicationsByQuery(apps, &qWithoutHealth) {
		stats.TotalByHealthStatus[app.Status.Health.Status]++
	}

	qWithoutAutoSync := *q
	qWithoutAutoSync.AutoSyncs = nil
	for _, app := range filterApplicationsByQuery(apps, &qWithoutAutoSync) {
		if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.IsAutomatedSyncEnabled() {
			stats.AutoSyncCount["Enabled"]++
		} else {
			stats.AutoSyncCount["Disabled"]++
		}
	}

	destinationSet := make(map[string]v1alpha1.ApplicationDestination)
	namespaceSet := make(map[string]bool)
	labelMap := make(map[string]map[string]bool)
	repoSet := make(map[string]bool)

	filtered := filterApplicationsByQuery(apps, q)
	stats.Total = int64(len(filtered))

	for _, app := range filtered {
		destKey := app.Spec.Destination.Server + "/" + app.Spec.Destination.Name + "/" + app.Spec.Destination.Namespace
		if _, exists := destinationSet[destKey]; !exists {
			destinationSet[destKey] = app.Spec.Destination
		}
		if app.Spec.Destination.Namespace != "" {
			namespaceSet[app.Spec.Destination.Namespace] = true
		}
		for key, value := range app.Labels {
			if labelMap[key] == nil {
				labelMap[key] = make(map[string]bool)
			}
			labelMap[key][value] = true
		}
		for _, source := range app.Spec.GetSources() {
			if source.RepoURL != "" {
				repoSet[source.RepoURL] = true
			}
		}
	}

	for _, dest := range destinationSet {
		stats.Destinations = append(stats.Destinations, dest)
	}
	sort.Slice(stats.Destinations, func(i, j int) bool {
		if stats.Destinations[i].Server != stats.Destinations[j].Server {
			return stats.Destinations[i].Server < stats.Destinations[j].Server
		}
		return stats.Destinations[i].Name < stats.Destinations[j].Name
	})

	for ns := range namespaceSet {
		stats.Namespaces = append(stats.Namespaces, ns)
	}
	sort.Strings(stats.Namespaces)

	for key, valuesMap := range labelMap {
		values := make([]string, 0, len(valuesMap))
		for value := range valuesMap {
			values = append(values, value)
		}
		sort.Strings(values)
		stats.Labels = append(stats.Labels, v1alpha1.ApplicationLabelStats{Key: key, Values: values})
	}
	sort.Slice(stats.Labels, func(i, j int) bool {
		return stats.Labels[i].Key < stats.Labels[j].Key
	})

	for repo := range repoSet {
		stats.Repos = append(stats.Repos, repo)
	}
	sort.Strings(stats.Repos)

	return stats
}
