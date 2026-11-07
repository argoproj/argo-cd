import {hashKey, keepPreviousData, queryOptions, useQuery, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {bufferTime, filter, repeat, retry} from 'rxjs/operators';

import {Paginate, PaginateProps} from '../../../shared/components';
import * as models from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import * as AppUtils from '../utils';
import {FilteredApp, getAppFilterResults} from './applications-filter';
import {showCreateFirstAppState} from './applications-list-empty-state';
import {createMatcher} from './applications-list-search';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';
import {computeAppsListFacets, computeAppsListStats} from './applications-list-stats';

// Loads every application the user can see, keeps it current through the watch stream, and filters on
// the client.

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;
const VISIBILITY_DEBOUNCE = 500;

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

// Sits under applicationQueries.list's prefix, so invalidating every application list also refreshes this one.
export const listViewQuery = (projects: string[], appNamespace: string, names?: string[]) =>
    queryOptions({
        queryKey: ['applications', 'list', 'view', {projects, appNamespace, names}],
        queryFn: () => {
            // Favorites-only mode with no favorites selected: an empty (but defined) names array means
            // "match nothing". The server treats an empty filter as "no filter", so short-circuit here to
            // avoid fetching and continuously watching every application only to hide them client-side.
            if (names && names.length === 0) {
                return Promise.resolve({items: [], metadata: {}} as models.ApplicationList);
            }
            return services.applications.list(projects, 'application', {appNamespace, fields: APP_LIST_FIELDS, names}) as Promise<models.ApplicationList>;
        }
    });

export function applyWatchEvents(list: models.ApplicationList, changes: models.ApplicationWatchEvent[]): models.ApplicationList {
    const items = list.items.slice();
    changes.forEach(change => {
        const index = items.findIndex(item => AppUtils.appInstanceName(item) === AppUtils.appInstanceName(change.application));
        if (change.type === 'DELETED') {
            if (index > -1) {
                items.splice(index, 1);
            }
        } else if (index > -1) {
            items[index] = change.application;
        } else {
            items.unshift(change.application);
        }
    });
    return {...list, items};
}

function usePageVisible(): boolean {
    const [visible, setVisible] = React.useState(() => !document.hidden);
    React.useEffect(() => {
        let timer: ReturnType<typeof setTimeout>;
        // Wait until the user stops switching tabs back and forth, so the list isn't restarted too often.
        const onChange = () => {
            clearTimeout(timer);
            timer = setTimeout(() => setVisible(!document.hidden), VISIBILITY_DEBOUNCE);
        };
        document.addEventListener('visibilitychange', onChange);
        return () => {
            clearTimeout(timer);
            document.removeEventListener('visibilitychange', onChange);
        };
    }, []);
    return visible;
}

function filterApplications(applications: models.Application[], pref: AppsListPreferences, search: string, searchRegex: boolean, hydratorEnabled: boolean) {
    const processedApps = applications.map(app => ({...app, isAppOfAppsPattern: app.status.summary?.isAppOfApps === true}));
    const filterResults: FilteredApp[] = getAppFilterResults(processedApps, pref, hydratorEnabled);
    const matchesSearch = createMatcher(search, searchRegex);
    return {
        filterResults,
        filteredApps: filterResults.filter(app => matchesSearch(app.metadata.name, app.metadata.namespace) && Object.values(app.filterResult).every(val => val))
    };
}

export function useApplicationsList({pref, appNamespace, hydratorEnabled}: AppsListSourceQuery): AppsListSourceResult {
    const queryClient = useQueryClient();
    const visible = usePageVisible();
    // The favorites list is only part of the query while the favorites filter is on, so starring an
    // application with the filter off must not restart the list and watch.
    const names = pref.showFavorites ? pref.favoritesAppList || [] : undefined;
    // Preferences arrive as fresh objects on every emission, so pin the query to its key's content.
    const keyHash = hashKey([pref.projectsFilter, appNamespace, names]);
    const options = React.useMemo(() => listViewQuery(pref.projectsFilter, appNamespace, names), [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    const {queryKey} = options;
    const watchQuery = React.useMemo(() => ({projects: pref.projectsFilter, appNamespace, names}), [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    // Keep the current list (and with it the filter sidebar) rendered while a filter change reloads the
    // data, instead of dropping to the loading placeholder. Don't load while hidden; on becoming visible
    // the stale list refetches, and isFetching is already set in that render, so the watch waits for it.
    const query = useQuery({...options, placeholderData: keepPreviousData, enabled: visible});
    const {refetch} = query;

    // Stream changes from the version the list was read at. Waiting while a fetch is in flight means a
    // reload restarts the watch from the new version, rather than resuming one that may be too old.
    const resourceVersion = query.isPlaceholderData ? undefined : query.data?.metadata?.resourceVersion;
    // A failed reload keeps the old data, but the server replays nothing to a watch given a version, so
    // resuming from it would miss every change since.
    const watching = !!resourceVersion && visible && !query.isFetching && !query.isError;
    React.useEffect(() => {
        if (!watching) {
            return;
        }
        const subscription = services.applications
            .watch('application', {projects: watchQuery.projects, appNamespace: watchQuery.appNamespace, resourceVersion}, {fields: APP_WATCH_FIELDS, names: watchQuery.names})
            .pipe(repeat())
            .pipe(retry({delay: WATCH_RETRY_TIMEOUT}))
            // batch events to avoid constant re-rendering and improve UI performance
            // The list is scoped to appNamespace, but the server only applies it to a watch for a single
            // application, so drop other namespaces' events rather than adding them to the list.
            .pipe(filter(change => !watchQuery.appNamespace || change.application.metadata.namespace === watchQuery.appNamespace))
            .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
            .pipe(filter(changes => changes.length > 0))
            .subscribe(changes => queryClient.setQueryData(queryKey, list => list && applyWatchEvents(list, changes)));
        return () => subscription.unsubscribe();
    }, [queryClient, queryKey, watchQuery, resourceVersion, watching]);

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
    const derived = React.useMemo(() => {
        const apps = applications || [];
        const {filteredApps, filterResults} = filterApplications(apps, pref, pref.search, pref.searchRegex, hydratorEnabled);
        return {
            rows: filteredApps,
            facets: computeAppsListFacets(filterResults),
            stats: computeAppsListStats(filteredApps),
            noApplications: showCreateFirstAppState(apps, pref),
            searchSuggestions: apps
        };
    }, [applications, pref, hydratorEnabled]);

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

/** Renders the list's rows through the paginator, which pages them on the client. */
export const AppsListPaginate = ({list, ...props}: {list: AppsListSourceResult} & Omit<PaginateProps<models.Application>, 'data'>) => <Paginate {...props} data={list.rows} />;
