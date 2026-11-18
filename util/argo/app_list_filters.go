package argo

import (
	"regexp"
	"strings"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// MatchesRepos returns true if any application source repo URL is in repos.
func MatchesRepos(app *argoappv1.Application, repos []string) bool {
	if len(repos) == 0 {
		return true
	}
	repoSet := make(map[string]bool, len(repos))
	for _, r := range repos {
		repoSet[r] = true
	}
	for _, source := range app.Spec.GetSources() {
		if repoSet[source.RepoURL] {
			return true
		}
	}
	return false
}

// MatchesClusters returns true if the application destination server or name matches.
func MatchesClusters(app *argoappv1.Application, clusters []string) bool {
	if len(clusters) == 0 {
		return true
	}
	for _, c := range clusters {
		if app.Spec.Destination.Server == c || app.Spec.Destination.Name == c {
			return true
		}
	}
	return false
}

// MatchesNamespaces returns true if the application destination namespace matches.
func MatchesNamespaces(app *argoappv1.Application, namespaces []string) bool {
	if len(namespaces) == 0 {
		return true
	}
	for _, ns := range namespaces {
		if app.Spec.Destination.Namespace == ns {
			return true
		}
	}
	return false
}

// MatchesLabels returns true if the application has every key=value label in labels.
// Entries without '=' match on key presence only.
func MatchesLabels(app *argoappv1.Application, labels []string) bool {
	if len(labels) == 0 {
		return true
	}
	if app.Labels == nil {
		return false
	}
	for _, entry := range labels {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			if _, exists := app.Labels[entry]; !exists {
				return false
			}
			continue
		}
		if app.Labels[key] != value {
			return false
		}
	}
	return true
}

// MatchesAutoSync returns true if the application's automated sync state is listed in autoSyncs.
// Expected values are "Enabled" and/or "Disabled".
func MatchesAutoSync(app *argoappv1.Application, autoSyncs []string) bool {
	if len(autoSyncs) == 0 {
		return true
	}
	enabled := app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil
	state := "Disabled"
	if enabled {
		state = "Enabled"
	}
	for _, v := range autoSyncs {
		if v == state {
			return true
		}
	}
	return false
}

// MatchesSyncStatuses returns true if the application sync status is in syncStatuses.
func MatchesSyncStatuses(app *argoappv1.Application, syncStatuses []string) bool {
	if len(syncStatuses) == 0 {
		return true
	}
	status := string(app.Status.Sync.Status)
	for _, s := range syncStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// MatchesHealthStatuses returns true if the application health status is in healthStatuses.
func MatchesHealthStatuses(app *argoappv1.Application, healthStatuses []string) bool {
	if len(healthStatuses) == 0 {
		return true
	}
	status := string(app.Status.Health.Status)
	for _, s := range healthStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// MatchesSearch returns true if search matches the application name or namespace.
// When useRegex is true, search is compiled as a regular expression; invalid patterns never match.
// When useRegex is false, matching is a case-sensitive substring (aligned with the UI SearchBar).
// Prefer MatchesSearchCompiled when filtering many apps so the pattern is compiled once.
func MatchesSearch(app *argoappv1.Application, search string, useRegex bool) bool {
	if search == "" {
		return true
	}
	if useRegex {
		re, err := regexp.Compile(search)
		if err != nil {
			return false
		}
		return MatchesSearchCompiled(app, "", re)
	}
	return MatchesSearchCompiled(app, search, nil)
}

// MatchesSearchCompiled matches name/namespace against a substring search or a precompiled regexp.
// When re is non-nil it takes precedence over search. An empty search with a nil re matches everything.
func MatchesSearchCompiled(app *argoappv1.Application, search string, re *regexp.Regexp) bool {
	name := app.Name
	ns := app.Namespace
	if re != nil {
		return re.MatchString(name) || re.MatchString(ns)
	}
	if search == "" {
		return true
	}
	return strings.Contains(name, search) || strings.Contains(ns, search)
}
