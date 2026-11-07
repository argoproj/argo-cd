import {hashKey, queryOptions} from '@tanstack/react-query';
import * as React from 'react';
import {Observable} from 'rxjs';

import {useWatchedList, WatchEvent} from '../../../shared/hooks/use-watched-list';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

// Loads every ApplicationSet the user can see and keeps it current through the watch stream.

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
    // Preferences arrive as fresh objects on every emission, so pin the query to its key's content.
    const keyHash = hashKey([projects]);
    const options = React.useMemo(() => appSetsListViewQuery(projects), [keyHash]); // eslint-disable-line react-hooks/exhaustive-deps
    const watch = React.useCallback(
        (resourceVersion: string) =>
            // The service puts ApplicationSet watch events in the application field.
            services.applications.watch('applicationset', {projects, resourceVersion}, {fields: APPSET_WATCH_FIELDS}) as unknown as Observable<WatchEvent<models.ApplicationSet>>,
        [keyHash] // eslint-disable-line react-hooks/exhaustive-deps
    );
    const query = useWatchedList(options, watch);
    const {refetch} = query;

    const appSets = query.data?.items;
    return {
        appSets: appSets || [],
        loading: !appSets && !query.isError,
        failed: query.isError,
        retry: () => refetch()
    };
}
