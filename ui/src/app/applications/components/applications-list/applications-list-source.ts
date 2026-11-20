import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';
import {AppsListStats} from './applications-list-stats';
import {ApplicationQueryValueIndex} from './applications-query-completion';

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
    stats: AppsListStats;
    /** Completion values from all authorized applications, before query filtering. */
    queryValues: ApplicationQueryValueIndex;
    /** The user has no applications at all, as opposed to none matching their filters. */
    noApplications: boolean;
    /** Marks an application as refreshing until its next update arrives. */
    markRefreshing: (appName: string, appNamespace: string) => void;
}
