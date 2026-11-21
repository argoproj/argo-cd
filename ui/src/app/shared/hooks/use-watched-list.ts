import {keepPreviousData, QueryKey, useQuery, useQueryClient, UseQueryOptions} from '@tanstack/react-query';
import * as React from 'react';
import {Observable} from 'rxjs';
import {bufferTime, filter, repeat, retry} from 'rxjs/operators';

import {appInstanceName} from '../components/app-utils';
import * as models from '../models';
import {usePageVisible} from './use-page-visible';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;
/** Coalesce watch-driven list invalidations so noisy DELETE batches do not List-thrash. */
export const WATCH_REFETCH_DEBOUNCE_MS = 2000;

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

export type ApplyWatchEventsResult<L> = {list: L; refetch?: boolean};

export interface UseWatchedListOptions<T extends models.AbstractApplication, L extends {items: T[]}> {
    /**
     * When set, the watch stream is keyed by this identity instead of the list queryKey. List
     * refetches (search/page/sort) keep the same stream; only a watchKey change restarts it.
     */
    watchKey?: string;
    /** Custom event applicator. Return refetch:true to reload the list query after applying. */
    applyEvents?: (list: L, changes: WatchEvent<T>[]) => ApplyWatchEventsResult<L>;
}

/**
 * Lists through react-query and keeps the cached list current with a watch started from the version the list
 * was read at. Pass `options` and `watch` memoized on the list's parameters: a new identity restarts both,
 * unless `watchKey` is provided to keep the stream stable across list-parameter changes.
 */
export function useWatchedList<T extends models.AbstractApplication, L extends {items: T[]; metadata?: {resourceVersion?: string}}, K extends QueryKey>(
    options: UseQueryOptions<L, Error, L, K>,
    watch: (resourceVersion: string) => Observable<WatchEvent<T>>,
    watchedListOpts?: UseWatchedListOptions<T, L>
) {
    const queryClient = useQueryClient();
    const visible = usePageVisible();
    // Keep the current list rendered while new parameters reload it, instead of dropping to the loading
    // placeholder. Don't load while hidden; on becoming visible the stale list refetches, and isFetching is
    // already set in that render, so the watch waits for it.
    const query = useQuery({...options, placeholderData: keepPreviousData, enabled: visible});

    const {queryKey} = options;
    // Keep latest list key / applicator for the watch subscription without reading refs during render.
    const queryKeyRef = React.useRef(queryKey);
    const applyEventsRef = React.useRef(watchedListOpts?.applyEvents);
    React.useEffect(() => {
        queryKeyRef.current = queryKey;
        applyEventsRef.current = watchedListOpts?.applyEvents;
    });

    const stableWatch = watchedListOpts?.watchKey !== undefined;
    const watchKey = watchedListOpts?.watchKey ?? '';

    // For stable watches, pin resourceVersion to the first successful list under this watchKey so
    // search/page refetches do not tear down the stream. Stored as state (not a ref) so render can
    // read it without violating react-hooks/refs.
    const [stablePin, setStablePin] = React.useState<{watchKey: string; resourceVersion: string} | null>(null);
    const listRV = !query.isPlaceholderData ? query.data?.metadata?.resourceVersion : undefined;
    if (stableWatch && stablePin?.watchKey !== watchKey) {
        if (listRV) {
            setStablePin({watchKey, resourceVersion: listRV});
        } else if (stablePin !== null) {
            setStablePin(null);
        }
    }

    const resourceVersion = stableWatch ? (stablePin?.watchKey === watchKey ? stablePin.resourceVersion : undefined) : listRV;
    // Stable watches keep streaming while a list refetch is in flight; legacy watches pause so the
    // stream always resumes from the version of the list that just loaded.
    const watching = !!resourceVersion && visible && !query.isError && (stableWatch || !query.isFetching);

    React.useEffect(() => {
        if (!watching) {
            return;
        }
        let refetchTimer: ReturnType<typeof setTimeout> | undefined;
        const scheduleRefetch = () => {
            if (refetchTimer !== undefined) {
                return;
            }
            refetchTimer = setTimeout(() => {
                refetchTimer = undefined;
                queryClient.invalidateQueries({queryKey: queryKeyRef.current});
            }, WATCH_REFETCH_DEBOUNCE_MS);
        };
        const subscription = watch(resourceVersion)
            .pipe(repeat())
            .pipe(retry({delay: WATCH_RETRY_TIMEOUT}))
            // batch events to avoid constant re-rendering and improve UI performance
            .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
            .pipe(filter(changes => changes.length > 0))
            .subscribe(changes => {
                let shouldRefetch = false;
                queryClient.setQueryData<L>(queryKeyRef.current, list => {
                    if (!list) {
                        return list;
                    }
                    if (applyEventsRef.current) {
                        const result = applyEventsRef.current(list, changes);
                        shouldRefetch = !!result.refetch;
                        return result.list;
                    }
                    return applyWatchEvents(list, changes);
                });
                if (shouldRefetch) {
                    scheduleRefetch();
                }
            });
        return () => {
            if (refetchTimer !== undefined) {
                clearTimeout(refetchTimer);
            }
            subscription.unsubscribe();
        };
        // watchKey replaces queryKey as the restart signal when stableWatch is on.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [queryClient, stableWatch ? watchKey : queryKey, watch, resourceVersion, watching]);

    return query;
}
