import {queryOptions} from '@tanstack/react-query';
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
// No project filter: the service only sends projects for applications.
export const appSetsListViewQuery = queryOptions({
    queryKey: ['applications', 'list', 'appset-view'],
    queryFn: () => services.applications.list([], 'applicationset', {fields: APPSET_LIST_FIELDS}) as Promise<models.ApplicationSetList>
});

// The service puts ApplicationSet watch events in the application field.
const watchAppSets = (resourceVersion: string) =>
    services.applications.watch('applicationset', {resourceVersion}, {fields: APPSET_WATCH_FIELDS}) as unknown as Observable<WatchEvent<models.ApplicationSet>>;

export interface ApplicationSetsListResult {
    appSets: models.ApplicationSet[];
    /** No data yet: render the loading placeholder. */
    loading: boolean;
    /** Loading failed, including a reload of data already shown. */
    failed: boolean;
    retry: () => void;
}

export function useApplicationSetsList(): ApplicationSetsListResult {
    const query = useWatchedList(appSetsListViewQuery, watchAppSets);
    const {refetch} = query;

    const appSets = query.data?.items;
    return {
        appSets: appSets || [],
        loading: !appSets && !query.isError,
        failed: query.isError,
        retry: () => refetch()
    };
}
