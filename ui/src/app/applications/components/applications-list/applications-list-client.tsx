import {hashKey, queryOptions, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {filter} from 'rxjs/operators';

import {Paginate, PaginateProps} from '../../../shared/components';
import {useWatchedList} from '../../../shared/hooks/use-watched-list';
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
    // The favorites list is only part of the query while the favorites filter is on, so starring an
    // application with the filter off must not restart the list and watch.
    const names = pref.showFavorites ? pref.favoritesAppList || [] : undefined;
    // Preferences arrive as fresh objects on every emission, so pin the query to its key's content.
    const keyHash = hashKey([pref.projectsFilter, appNamespace, names]);
    const options = React.useMemo(() => listViewQuery(pref.projectsFilter, appNamespace, names), [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    const {queryKey} = options;
    const watch = React.useCallback(
        (resourceVersion: string) =>
            services.applications
                .watch('application', {projects: pref.projectsFilter, appNamespace, resourceVersion}, {fields: APP_WATCH_FIELDS, names})
                // The list is scoped to appNamespace, but the server only applies it to a watch for a single
                // application, so drop other namespaces' events rather than adding them to the list.
                .pipe(filter(change => !appNamespace || change.application.metadata.namespace === appNamespace)),
        [keyHash] // eslint-disable-line react-hooks/exhaustive-deps
    );
    const query = useWatchedList(options, watch);
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
