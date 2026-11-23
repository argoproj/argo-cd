import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';

function hasActiveFilter(values?: string[]): boolean {
    return (values || []).length > 0;
}

/**
 * Returns true when an empty list means the user has no applications at all, rather than none that
 * pass their filters/search. Filters (and SSP search) are applied before the UI sees the list, so
 * an empty response with any constraint active only means "nothing matched". Getting this wrong
 * shows the create-first empty state and hides the filter sidebar, which leaves the user unable
 * to clear the constraint.
 **/
export function showCreateFirstAppState(apps: Application[], pref: AppsListPreferences, search?: string): boolean {
    if (apps.length > 0) {
        return false;
    }
    if (search && search.trim() !== '') {
        return false;
    }
    if (pref.showFavorites) {
        return false;
    }
    // Keep in sync with AppsListPreferences.clearFilters / AbstractAppsListPreferences.clearFilters.
    return !(
        hasActiveFilter(pref.projectsFilter) ||
        hasActiveFilter(pref.labelsFilter) ||
        hasActiveFilter(pref.annotationsFilter) ||
        hasActiveFilter(pref.healthFilter) ||
        hasActiveFilter(pref.clustersFilter) ||
        hasActiveFilter(pref.namespacesFilter) ||
        hasActiveFilter(pref.reposFilter) ||
        hasActiveFilter(pref.targetRevisionFilter) ||
        hasActiveFilter(pref.syncFilter) ||
        hasActiveFilter(pref.autoSyncFilter) ||
        hasActiveFilter(pref.operationFilter) ||
        hasActiveFilter(pref.hydrationFilter)
    );
}
