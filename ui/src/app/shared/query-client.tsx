import {QueryCache, QueryClient} from '@tanstack/react-query';
import {ErrorNotification, NotificationsApi, NotificationType} from 'argo-ui';
import * as React from 'react';

declare module '@tanstack/react-query' {
    interface Register {
        queryMeta: {silent?: boolean};
    }
}

// Defaults mirror argo-ui's DataLoader, which this replaces: no retries, no focus refetch, and a
// notification for every failed load except 401 (handled by the login redirect) or queries marked silent.
export function newQueryClient(notifications: NotificationsApi): QueryClient {
    return new QueryClient({
        queryCache: new QueryCache({
            onError: (e: any, query) => {
                if (e?.status !== 401 && !query.meta?.silent) {
                    notifications.show({content: <ErrorNotification title='Unable to load data' e={e} />, type: NotificationType.Error});
                }
            }
        }),
        defaultOptions: {queries: {retry: false, refetchOnWindowFocus: false}}
    });
}

// Same markup DataLoader rendered while loading and on failure.
export const loading = <p style={{padding: '0.5em', textAlign: 'center'}}>Loading...</p>;

export function loadFailed(retry: () => void) {
    return (
        <p style={{padding: '0.5em', textAlign: 'center'}}>
            Failed to load data, please <a onClick={() => retry()}>try again</a>.
        </p>
    );
}
