import * as React from 'react';
import {QueryClient, QueryClientProvider, QueryObserver} from '@tanstack/react-query';
import {act, renderHook} from '@testing-library/react';

import {useClusterMutations, useProjectMutations} from './mutations';
import {clusterQueries, projectQueries} from './queries';
import {services} from './services';

jest.mock('./services', () => ({
    services: {
        projects: {delete: jest.fn(() => Promise.resolve(true)), updateRole: jest.fn(() => Promise.resolve({}))},
        clusters: {delete: jest.fn(() => Promise.reject(new Error('boom')))}
    }
}));

const setup = <T,>(hook: () => T) => {
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    return {client, result: renderHook(hook, {wrapper}).result};
};

describe('mutations', () => {
    beforeEach(() => jest.clearAllMocks());

    it('resolves only after the invalidated queries have refetched', async () => {
        const {client, result} = setup(() => useProjectMutations('p'));
        const queryKey = [...projectQueries.byName('p'), 'detailed'];
        // Invalidation only refetches queries with an observer, so keep one subscribed.
        const observer = new QueryObserver(client, {
            queryKey,
            queryFn: jest
                .fn()
                .mockResolvedValueOnce('old')
                .mockReturnValueOnce(new Promise(resolve => setTimeout(() => resolve('new'), 10)))
        });
        const unsubscribe = observer.subscribe(() => undefined);
        await act(() => observer.refetch());

        // The delayed refetch and the read inside act make an unawaited invalidation observable.
        let seen: unknown;
        await act(async () => {
            await result.current.updateRole.mutateAsync({} as any);
            seen = client.getQueryData(queryKey);
        });

        expect(seen).toBe('new');
        unsubscribe();
    });

    it('does not refetch project details after deleting the project', async () => {
        const {client, result} = setup(() => useProjectMutations('p'));
        client.setQueryData(projectQueries.list().queryKey, []);
        client.setQueryData([...projectQueries.byName('p'), 'detailed'], {});

        await act(() => result.current.delete.mutateAsync());

        expect(client.getQueryState(projectQueries.list().queryKey).isInvalidated).toBe(true);
        expect(client.getQueryState([...projectQueries.byName('p'), 'detailed']).isInvalidated).toBe(false);
    });

    it('refreshes the cluster list after a failed delete', async () => {
        const {client, result} = setup(() => useClusterMutations());
        client.setQueryData(clusterQueries.list().queryKey, []);

        await act(() => result.current.delete.mutateAsync('https://c').catch((): void => undefined));

        expect(services.clusters.delete).toHaveBeenCalledWith('https://c');
        expect(client.getQueryState(clusterQueries.list().queryKey).isInvalidated).toBe(true);
    });
});
