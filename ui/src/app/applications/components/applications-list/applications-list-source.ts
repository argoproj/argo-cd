import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';
import {AppsListFacets, AppsListStats} from './applications-list-stats';

// Contract between the applications list and the hook that loads it (see applications-list-client.tsx,
// which also provides the paginator for the rows).
// The list renders only what this returns, so the loading strategy can change without touching the view.

export interface AppsListSourceQuery {
    pref: AppsListPreferences & {page: number; search: string};
    appNamespace: string;
    hydratorEnabled: boolean;
}

export interface AppsListSourceResult {
    /** No data yet: render the loading placeholder. */
    loading: boolean;
    /** Loading failed, including a reload of data already shown. */
    failed: boolean;
    retry: () => void;
    /** The applications matching the filters and search, in the order to render them. */
    rows: Application[];
    facets: AppsListFacets;
    stats: AppsListStats;
    /** The user has no applications at all, as opposed to none matching their filters. */
    noApplications: boolean;
    /** Applications offered by the search bar's autocomplete. */
    searchSuggestions: Application[];
    /** Marks an application as refreshing until its next update arrives. */
    markRefreshing: (appName: string, appNamespace: string) => void;
}
