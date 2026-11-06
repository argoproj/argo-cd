import {hashKey, keepPreviousData, queryOptions, useQuery, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {bufferTime, filter, repeat, retry} from 'rxjs/operators';

import {usePageVisible} from '../../../shared/hooks/use-page-visible';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {applyWatchEvents} from './applications-list-client';

// Loads every ApplicationSet the user can see and keeps it current through the watch stream.

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

const APPSET_FIELDS = [
    'metadata.name',
    'metadata.namespace',
    'metadata.annotations',
    'metadata.labels',
    'metadata.creationTimestamp',
    'metadata.deletionTimestamp',
    'spec',
    'status.conditions',
    'status.resources',
    'status.resourcesCount',
    'status.health'
];
const APPSET_LIST_FIELDS = ['metadata.resourceVersion', ...APPSET_FIELDS.map(field => `items.${field}`)];
const APPSET_WATCH_FIELDS = ['result.type', ...APPSET_FIELDS.map(field => `result.applicationSet.${field}`)];

// Sits under applicationQueries.list's prefix, so invalidating every application list also refreshes this one.
export const appSetsListViewQuery = (projects: string[]) =>
    queryOptions({
        queryKey: ['applications', 'list', 'appset-view', {projects}],
        queryFn: () => services.applications.list(projects, 'applicationset', {fields: APPSET_LIST_FIELDS}) as Promise<models.ApplicationSetList>
    });

export interface ApplicationSetsListResult {
    appSets: models.ApplicationSet[];
    /** No data yet: render the loading placeholder. */
    loading: boolean;
    /** Loading failed, including a reload of data already shown. */
    failed: boolean;
    retry: () => void;
}

export function useApplicationSetsList(projects: string[]): ApplicationSetsListResult {
    const queryClient = useQueryClient();
    const visible = usePageVisible();
    // Preferences arrive as fresh objects on every emission, so pin the query to its key's content.
    const keyHash = hashKey([projects]);
    const options = React.useMemo(() => appSetsListViewQuery(projects), [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    const {queryKey} = options;
    const watchProjects = React.useMemo(() => projects, [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    // Don't load while hidden; on becoming visible the stale list refetches, and isFetching is already set
    // in that render, so the watch waits for it.
    const query = useQuery({...options, placeholderData: keepPreviousData, enabled: visible});
    const {refetch} = query;

    // Stream changes from the version the list was read at, and never from a failed reload's old version:
    // the server replays nothing to a watch given a version.
    const resourceVersion = query.isPlaceholderData ? undefined : query.data?.metadata?.resourceVersion;
    const watching = !!resourceVersion && visible && !query.isFetching && !query.isError;
    React.useEffect(() => {
        if (!watching) {
            return;
        }
        const subscription = services.applications
            .watch('applicationset', {projects: watchProjects, resourceVersion}, {fields: APPSET_WATCH_FIELDS})
            .pipe(repeat())
            .pipe(retry({delay: WATCH_RETRY_TIMEOUT}))
            // batch events to avoid constant re-rendering and improve UI performance
            .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
            .pipe(filter(changes => changes.length > 0))
            .subscribe(changes =>
                queryClient.setQueryData(queryKey, list => list && applyWatchEvents(list, changes as unknown as {type: models.WatchType; application: models.ApplicationSet}[]))
            );
        return () => subscription.unsubscribe();
    }, [queryClient, queryKey, watchProjects, resourceVersion, watching]);

    const appSets = query.data?.items;
    return {
        appSets: appSets || [],
        loading: !appSets && !query.isError,
        failed: query.isError,
        retry: () => refetch()
    };
}
