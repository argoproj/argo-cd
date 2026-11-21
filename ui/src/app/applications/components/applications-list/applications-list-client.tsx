import {queryOptions, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {filter} from 'rxjs/operators';

import {Paginate, PaginateProps} from '../../../shared/components';
import {useWatchedList} from '../../../shared/hooks/use-watched-list';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as AppUtils from '../utils';
import {compileApplicationQuery} from './applications-query';
import {indexApplicationQueryValues} from './applications-query-completion';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';
import {computeAppsListStats} from './applications-list-stats';

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
export const listViewQuery = (appNamespace: string) =>
    queryOptions({
        queryKey: ['applications', 'list', 'view', {appNamespace}],
        queryFn: () => services.applications.list([], 'application', {appNamespace, fields: APP_LIST_FIELDS}) as Promise<models.ApplicationList>
    });

export function useApplicationsList({pref, appNamespace}: AppsListSourceQuery): AppsListSourceResult {
    const queryClient = useQueryClient();
    // Load the whole authorized scope. A project or favorite clause can be inside OR, so narrowing
    // the server request by local preferences would silently remove valid matches from shared queries.
    const options = React.useMemo(() => listViewQuery(appNamespace), [appNamespace]);
    const {queryKey} = options;
    const watch = React.useCallback(
        (resourceVersion: string) =>
            services.applications
                .watch('application', {projects: [], appNamespace, resourceVersion}, {fields: APP_WATCH_FIELDS})
                // The list is scoped to appNamespace, but the server only applies it to a watch for a single
                // application, so drop other namespaces' events rather than adding them to the list.
                .pipe(filter(change => !appNamespace || change.application.metadata.namespace === appNamespace)),
        [appNamespace]
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
    const queryValues = React.useMemo(() => indexApplicationQueryValues(applications || []), [applications]);
    const applicationQuery = React.useMemo(() => compileApplicationQuery(pref.search, {favorites: pref.favoritesAppList}), [pref.search, pref.favoritesAppList]);
    const derived = React.useMemo(() => {
        const apps = applications || [];
        const filteredApps = apps.filter(applicationQuery.matches).map(app => ({...app, isAppOfAppsPattern: app.status.summary?.isAppOfApps === true}));
        return {
            rows: filteredApps,
            stats: computeAppsListStats(filteredApps),
            noApplications: apps.length === 0
        };
    }, [applications, applicationQuery]);

    return {
        loading: !applications && !query.isError,
        // A failed reload shows the error rather than the list it would have replaced, which the watch no
        // longer keeps current.
        failed: query.isError,
        retry: () => refetch(),
        markRefreshing,
        queryValues,
        ...derived
    };
}

/** Renders the list's rows through the paginator, which pages them on the client. */
export const AppsListPaginate = ({list, ...props}: {list: AppsListSourceResult} & Omit<PaginateProps<models.Application>, 'data'>) => <Paginate {...props} data={list.rows} />;
