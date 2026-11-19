import {keepPreviousData, QueryKey, useQuery, useQueryClient, UseQueryOptions} from '@tanstack/react-query';
import * as React from 'react';
import {Observable} from 'rxjs';
import {bufferTime, filter, repeat, retry} from 'rxjs/operators';

import {appInstanceName} from '../components/app-utils';
import * as models from '../models';
import {usePageVisible} from './use-page-visible';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

export interface WatchEvent<T> {
    type: models.WatchType;
    application: T;
}

export function applyWatchEvents<T extends models.AbstractApplication, L extends {items: T[]}>(list: L, changes: WatchEvent<T>[]): L {
    const items = list.items.slice();
    changes.forEach(change => {
        const index = items.findIndex(item => appInstanceName(item) === appInstanceName(change.application));
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

/**
 * Lists through react-query and keeps the cached list current with a watch started from the version the list
 * was read at. Pass `options` and `watch` memoized on the list's parameters: a new identity restarts both.
 */
export function useWatchedList<T extends models.AbstractApplication, L extends {items: T[]; metadata?: {resourceVersion?: string}}, K extends QueryKey>(
    options: UseQueryOptions<L, Error, L, K>,
    watch: (resourceVersion: string) => Observable<WatchEvent<T>>
) {
    const queryClient = useQueryClient();
    const visible = usePageVisible();
    // Keep the current list rendered while new parameters reload it, instead of dropping to the loading
    // placeholder. Don't load while hidden; on becoming visible the stale list refetches, and isFetching is
    // already set in that render, so the watch waits for it.
    const query = useQuery({...options, placeholderData: keepPreviousData, enabled: visible});

    // Waiting while a fetch is in flight means a reload restarts the watch from the new version, rather than
    // resuming one that may be too old. A failed reload keeps the old data, but the server replays nothing to
    // a watch given a version, so resuming from it would miss every change since.
    const resourceVersion = query.isPlaceholderData ? undefined : query.data?.metadata?.resourceVersion;
    const watching = !!resourceVersion && visible && !query.isFetching && !query.isError;
    const {queryKey} = options;
    React.useEffect(() => {
        if (!watching) {
            return;
        }
        const subscription = watch(resourceVersion)
            .pipe(repeat())
            .pipe(retry({delay: WATCH_RETRY_TIMEOUT}))
            // batch events to avoid constant re-rendering and improve UI performance
            .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
            .pipe(filter(changes => changes.length > 0))
            .subscribe(changes => queryClient.setQueryData<L>(queryKey, list => list && applyWatchEvents(list, changes)));
        return () => subscription.unsubscribe();
    }, [queryClient, queryKey, watch, resourceVersion, watching]);

    return query;
}
