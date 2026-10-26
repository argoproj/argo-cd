import {QueryCache, QueryClient} from '@tanstack/react-query';
import {ErrorNotification, NotificationsApi, NotificationType} from 'argo-ui';
import * as React from 'react';

// Defaults mirror argo-ui's DataLoader, which this replaces: no retries, no focus refetch, and a
// notification for every failed load except 401 (handled by the login redirect).
export function newQueryClient(notifications: NotificationsApi): QueryClient {
    return new QueryClient({
        queryCache: new QueryCache({
            onError: (e: any) => {
                if (e?.status !== 401) {
                    notifications.show({content: <ErrorNotification title='Unable to load data' e={e} />, type: NotificationType.Error});
                }
            }
        }),
        defaultOptions: {queries: {retry: false, refetchOnWindowFocus: false}}
    });
}
