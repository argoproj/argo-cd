import {hashKey, queryOptions, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {filter} from 'rxjs/operators';

import {Paginate, PaginateProps} from '../../../shared/components';
import {AuthSettingsCtx} from '../../../shared/context';
import {ApplyWatchEventsResult, useWatchedList, WatchEvent} from '../../../shared/hooks/use-watched-list';
import * as models from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import * as AppUtils from '../utils';
import {FilteredApp, getAppFilterResults} from './applications-filter';
import {showCreateFirstAppState} from './applications-list-empty-state';
import {createMatcher} from './applications-list-search';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';
import {AppsListFacets, AppsListStats, computeAppsListFacets, computeAppsListStats} from './applications-list-stats';

// Loads every application the user can see, keeps it current through the watch stream, and filters on
// the client — unless server-side pagination is enabled via the API server feature flag.

// The applications list/watch API supports only selected set of fields.
// Make sure to register any new fields in the `appFields` map of `pkg/apiclient/application/forwarder_overwrite.go`.
const APP_FIELDS = [
    'metadata.name',
    'metadata.namespace',
    'metadata.annotations',
    'metadata.labels',
    'metadata.creationTimestamp',
    'metadata.deletionTimestamp',
    'spec.destination',
    'spec.project',
    'spec.source',
    'spec.sources',
    'spec.sourceHydrator',
    'spec.syncPolicy',
    'operation.sync',
    'status.sourceHydrator',
    'status.summary',
    'status.sync.status',
    'status.sync.revision',
    'status.health',
    'status.operationState.phase',
    'status.operationState.operation.sync',
    'status.operationState.startedAt',
    'status.operationState.finishedAt'
];
const APP_LIST_FIELDS = ['metadata.resourceVersion', ...APP_FIELDS.map(field => `items.${field}`)];
const APP_WATCH_FIELDS = ['result.type', ...APP_FIELDS.map(field => `result.application.${field}`)];

export const APPS_LIST_PREFERENCES_KEY = 'applications-list';

const SORT_TITLE_TO_SERVER: {[title: string]: string} = {
    'Name': 'name',
    'Created At': 'createdAt',
    'Synchronized': 'synchronizedAt'
};

export function resolveApplicationPageSize(prefPageSize: number | undefined, serverDefault: number): number {
    const fallback = Math.max(1, serverDefault || 250);
    if (prefPageSize === undefined || prefPageSize === null || prefPageSize === -1 || prefPageSize <= 0) {
        return fallback;
    }
    return prefPageSize;
}

export function sortTitleToServer(sortTitle?: string): string | undefined {
    if (!sortTitle) {
        return undefined;
    }
    return SORT_TITLE_TO_SERVER[sortTitle] || undefined;
}

function stableFiltersFromPref(pref: AppsListPreferences & {page: number; search: string}) {
    const labels = (pref.labelsFilter || []).map(l => (l.includes('=') ? l : `${l}`));
    return {
        repos: pref.reposFilter?.length ? pref.reposFilter : undefined,
        clusters: pref.clustersFilter?.length ? pref.clustersFilter : undefined,
        namespaces: pref.namespacesFilter?.length ? pref.namespacesFilter : undefined,
        labels: labels.length ? labels : undefined,
        autoSyncs: pref.autoSyncFilter?.length ? pref.autoSyncFilter : undefined,
        syncStatuses: pref.syncFilter?.length ? pref.syncFilter : undefined,
        healthStatuses: pref.healthFilter?.length ? pref.healthFilter : undefined
    };
}

function prefToListOpts(pref: AppsListPreferences & {page: number; search: string}, pageSize: number, sortBy?: string) {
    return {
        ...stableFiltersFromPref(pref),
        search: pref.search || undefined,
        searchRegex: pref.searchRegex || undefined,
        sortBy: sortBy || undefined,
        offset: Math.max(0, (pref.page || 0) * pageSize),
        limit: pageSize
    };
}

function statsFromServer(serverStats?: models.ApplicationListStats, pageApps: models.Application[] = []): AppsListStats {
    const base = computeAppsListStats(pageApps);
    if (!serverStats) {
        return base;
    }
    return {
        ...base,
        total: serverStats.total ?? base.total,
        totalBySyncStatus: serverStats.totalBySyncStatus || base.totalBySyncStatus,
        totalByHealthStatus: serverStats.totalByHealthStatus || base.totalByHealthStatus,
        destinations: (serverStats.destinations || []).map(d => ({name: d.name, server: d.server})),
        namespaces: serverStats.namespaces || base.namespaces
    };
}

function facetsFromServer(serverStats?: models.ApplicationListStats, filterResults: FilteredApp[] = []): AppsListFacets {
    const clientFacets = computeAppsListFacets(filterResults);
    if (!serverStats) {
        return clientFacets;
    }
    return {
        ...clientFacets,
        totalBySyncStatus: serverStats.totalBySyncStatus || clientFacets.totalBySyncStatus,
        totalByHealthStatus: serverStats.totalByHealthStatus || clientFacets.totalByHealthStatus,
        totalByAutoSyncStatus: serverStats.autoSyncCount || clientFacets.totalByAutoSyncStatus,
        destinations: (serverStats.destinations || []).map(d => ({name: d.name, server: d.server})),
        namespaces: serverStats.namespaces || clientFacets.namespaces,
        labels: serverStats.labels || clientFacets.labels,
        repos: serverStats.repos?.length ? serverStats.repos : clientFacets.repos
    };
}

// Sits under applicationQueries.list's prefix, so invalidating every application list also refreshes this one.
export const listViewQuery = (projects: string[], appNamespace: string, names?: string[], listOpts?: ReturnType<typeof prefToListOpts>) =>
    queryOptions({
        queryKey: ['applications', 'list', 'view', {projects, appNamespace, names, listOpts}],
        queryFn: () => {
            // Favorites-only mode with no favorites selected: an empty (but defined) names array means
            // "match nothing". The server treats an empty filter as "no filter", so short-circuit here to
            // avoid fetching and continuously watching every application only to hide them client-side.
            if (names && names.length === 0) {
                return Promise.resolve({items: [], metadata: {}} as models.ApplicationList);
            }
            return services.applications.list(projects, 'application', {
                appNamespace,
                fields: APP_LIST_FIELDS,
                names,
                ...listOpts
            }) as Promise<models.ApplicationList>;
        }
    });

function filterApplications(applications: models.Application[], pref: AppsListPreferences, search: string, searchRegex: boolean, hydratorEnabled: boolean) {
    const processedApps = applications.map(app => ({...app, isAppOfAppsPattern: app.status.summary?.isAppOfApps === true}));
    const filterResults: FilteredApp[] = getAppFilterResults(processedApps, pref, hydratorEnabled);
    const matchesSearch = createMatcher(search, searchRegex);
    return {
        filterResults,
        filteredApps: filterResults.filter(app => matchesSearch(app.metadata.name, app.metadata.namespace) && Object.values(app.filterResult).every(val => val))
    };
}

/**
 * Applies watch events to a server-paged list without unshifting foreign-page apps into the view.
 *
 * Refetch is reserved for DELETED rows already on the page (to refill the page). ADDED and
 * off-page MODIFIED are ignored so noisy clusters do not invalidate the list on every status tick.
 */
export function applyServerSideWatchEvents(
    list: models.ApplicationList,
    changes: WatchEvent<models.Application>[],
    matchesSearch: (name: string, namespace: string) => boolean
): ApplyWatchEventsResult<models.ApplicationList> {
    const items = list.items.slice();
    let refetch = false;
    changes.forEach(change => {
        const app = change.application;
        const index = items.findIndex(item => AppUtils.appInstanceName(item) === AppUtils.appInstanceName(app));
        const matches = matchesSearch(app.metadata.name, app.metadata.namespace || '');
        if (change.type === 'DELETED') {
            if (index > -1) {
                items.splice(index, 1);
                refetch = true;
            }
            return;
        }
        if (change.type === 'ADDED') {
            // Do not unshift or refetch: new apps appear on the next intentional List (page/search/filter).
            return;
        }
        // MODIFIED — only touch apps already on this page.
        if (index < 0) {
            return;
        }
        if (!matches) {
            items.splice(index, 1);
            return;
        }
        items[index] = app;
    });
    return {list: {...list, items}, refetch};
}

function readAppsListViewPrefs(): {pageSizePref?: number; sortTitle?: string} {
    try {
        services.viewPreferences.init();
        const p = services.viewPreferences.getCurrentPreferences();
        return {
            pageSizePref: p.pageSizes?.[APPS_LIST_PREFERENCES_KEY],
            sortTitle: p.sortOptions?.[APPS_LIST_PREFERENCES_KEY]
        };
    } catch {
        return {};
    }
}

function useAppsListViewPrefs() {
    const [pageSizePref, setPageSizePref] = React.useState(() => readAppsListViewPrefs().pageSizePref);
    const [sortTitle, setSortTitle] = React.useState(() => readAppsListViewPrefs().sortTitle);
    React.useEffect(() => {
        services.viewPreferences.init();
        const sub = services.viewPreferences.getPreferences().subscribe(p => {
            setPageSizePref(p.pageSizes?.[APPS_LIST_PREFERENCES_KEY]);
            setSortTitle(p.sortOptions?.[APPS_LIST_PREFERENCES_KEY]);
        });
        return () => sub.unsubscribe();
    }, []);
    return {pageSizePref, sortTitle};
}

export function useApplicationsList({pref, appNamespace, hydratorEnabled}: AppsListSourceQuery): AppsListSourceResult {
    const queryClient = useQueryClient();
    const authSettings = React.useContext(AuthSettingsCtx);
    const serverSidePaginationEnabled = !!authSettings?.serverSidePaginationEnabled;
    const {pageSizePref, sortTitle} = useAppsListViewPrefs();
    const pageSize = serverSidePaginationEnabled
        ? resolveApplicationPageSize(pageSizePref, authSettings?.applicationPageSize || 250)
        : Math.max(1, authSettings?.applicationPageSize || 250);
    const sortBy = serverSidePaginationEnabled ? sortTitleToServer(sortTitle) : undefined;

    // The favorites list is only part of the query while the favorites filter is on, so starring an
    // application with the filter off must not restart the list and watch.
    const names = pref.showFavorites ? pref.favoritesAppList || [] : undefined;
    const watchFilters = serverSidePaginationEnabled ? stableFiltersFromPref(pref) : undefined;
    const listOpts = serverSidePaginationEnabled ? prefToListOpts(pref, pageSize, sortBy) : undefined;
    // Preferences arrive as fresh objects on every emission, so pin the query to its key's content.
    const listKeyHash = hashKey([pref.projectsFilter, appNamespace, names, listOpts]);
    const watchKeyHash = serverSidePaginationEnabled ? hashKey([pref.projectsFilter, appNamespace, names, watchFilters]) : listKeyHash;
    const options = React.useMemo(() => listViewQuery(pref.projectsFilter, appNamespace, names, listOpts), [listKeyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    const {queryKey} = options;

    const watch = React.useCallback(
        (resourceVersion: string) =>
            services.applications
                .watch(
                    'application',
                    {projects: pref.projectsFilter, appNamespace, resourceVersion},
                    {
                        fields: APP_WATCH_FIELDS,
                        names,
                        ...(serverSidePaginationEnabled
                            ? {
                                  repos: watchFilters?.repos,
                                  clusters: watchFilters?.clusters,
                                  namespaces: watchFilters?.namespaces,
                                  labels: watchFilters?.labels,
                                  autoSyncs: watchFilters?.autoSyncs,
                                  syncStatuses: watchFilters?.syncStatuses,
                                  healthStatuses: watchFilters?.healthStatuses
                              }
                            : {})
                    }
                )
                // The list is scoped to appNamespace, but the server only applies it to a watch for a single
                // application, so drop other namespaces' events rather than adding them to the list.
                .pipe(filter(change => !appNamespace || change.application.metadata.namespace === appNamespace)),
        [watchKeyHash] // eslint-disable-line react-hooks/exhaustive-deps
    );

    const applyEvents = React.useCallback(
        (list: models.ApplicationList, changes: WatchEvent<models.Application>[]) => applyServerSideWatchEvents(list, changes, createMatcher(pref.search, pref.searchRegex)),
        [pref.search, pref.searchRegex]
    );

    const query = useWatchedList(
        options,
        watch,
        serverSidePaginationEnabled
            ? {
                  watchKey: watchKeyHash,
                  applyEvents
              }
            : undefined
    );
    const {refetch} = query;

    const markRefreshing = React.useCallback(
        (appName: string, appNs: string) => {
            // app refreshing might be done too quickly so that UI might miss it due to event batching
            // add refreshing annotation in the UI to improve user experience
            queryClient.setQueryData(queryKey, list => {
                if (!list) {
                    return list;
                }
                const items = list.items.map(app => {
                    if (app.metadata.name !== appName || app.metadata.namespace !== appNs) {
                        return app;
                    }
                    const refreshing = {...app, metadata: {...app.metadata, annotations: {...app.metadata.annotations}}};
                    AppUtils.setAppRefreshing(refreshing);
                    return refreshing;
                });
                return {...list, items};
            });
            services.applications.get(appName, appNs, 'application', 'normal');
        },
        [queryClient, queryKey]
    );

    const applications = query.data?.items;
    const serverStats = query.data?.stats;
    const derived = React.useMemo(() => {
        const apps = applications || [];
        if (serverSidePaginationEnabled) {
            const processedApps = apps.map(app => ({...app, isAppOfAppsPattern: app.status.summary?.isAppOfApps === true}));
            const filterResults: FilteredApp[] = getAppFilterResults(processedApps, pref, hydratorEnabled);
            return {
                rows: processedApps,
                facets: facetsFromServer(serverStats, filterResults),
                stats: statsFromServer(serverStats, processedApps),
                noApplications: showCreateFirstAppState(apps, pref, pref.search) && !(serverStats?.total > 0),
                searchSuggestions: apps
            };
        }
        const {filteredApps, filterResults} = filterApplications(apps, pref, pref.search, pref.searchRegex, hydratorEnabled);
        return {
            rows: filteredApps,
            facets: computeAppsListFacets(filterResults),
            stats: computeAppsListStats(filteredApps),
            noApplications: showCreateFirstAppState(apps, pref),
            searchSuggestions: apps
        };
    }, [applications, pref, hydratorEnabled, serverSidePaginationEnabled, serverStats]);

    return {
        loading: !applications && !query.isError,
        // A failed reload shows the error rather than the list it would have replaced, which the watch no
        // longer keeps current.
        failed: query.isError,
        retry: () => refetch(),
        markRefreshing,
        ...derived
    };
}

/** Renders the list's rows through the paginator, which pages them on the client (legacy) or passes through server pages. */
export const AppsListPaginate = ({list, ...props}: {list: AppsListSourceResult} & Omit<PaginateProps<models.Application>, 'data'>) => {
    const authSettings = React.useContext(AuthSettingsCtx);
    const {pageSizePref} = useAppsListViewPrefs();
    if (authSettings?.serverSidePaginationEnabled) {
        const pageSize = resolveApplicationPageSize(pageSizePref, authSettings.applicationPageSize || 250);
        return (
            <Paginate
                {...props}
                data={list.rows}
                totalItems={list.stats.total ?? list.rows.length}
                pageSizeOverride={pageSize}
                serverSorted={true}
                pageSizeOptions={[5, 10, 15, 20, 50, 100, 250]}
            />
        );
    }
    return <Paginate {...props} data={list.rows} />;
};
