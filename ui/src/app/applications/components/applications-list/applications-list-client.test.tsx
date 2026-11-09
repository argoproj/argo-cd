import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, render, renderHook, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import * as React from 'react';
import {of, Subject} from 'rxjs';

import {AnnotationRefreshKey, Application, ApplicationWatchEvent} from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import {AppsListPaginate, useApplicationsList} from './applications-list-client';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';

jest.mock('../../../shared/services', () => {
    const actual = jest.requireActual('../../../shared/services');
    return {...actual, services: {...actual.services, applications: {list: jest.fn(), watch: jest.fn(), get: jest.fn()}}};
});

const app = (name: string, health = 'Healthy') =>
    ({
        metadata: {name, namespace: 'argocd'},
        spec: {destination: {server: 'https://kubernetes.default.svc'}, source: {}},
        status: {sync: {status: 'Synced'}, health: {status: health}}
    }) as unknown as Application;

const pref = (overrides: Partial<AppsListPreferences> = {}) =>
    ({
        syncFilter: [],
        autoSyncFilter: [],
        healthFilter: [],
        hydrationFilter: [],
        namespacesFilter: [],
        clustersFilter: [],
        reposFilter: [],
        targetRevisionFilter: [],
        labelsFilter: [],
        annotationsFilter: [],
        operationFilter: [],
        projectsFilter: [],
        showFavorites: false,
        favoritesAppList: [],
        page: 0,
        search: '',
        ...overrides
    }) as AppsListSourceQuery['pref'];

let watch: Subject<ApplicationWatchEvent>;

const setup = (query: AppsListSourceQuery) => {
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    return renderHook(() => useApplicationsList(query), {wrapper}).result;
};

beforeEach(() => {
    jest.clearAllMocks();
    watch = new Subject<ApplicationWatchEvent>();
    jest.mocked(services.applications.watch).mockReturnValue(watch);
    jest.mocked(services.applications.list).mockResolvedValue({items: [app('a'), app('b', 'Degraded')], metadata: {resourceVersion: '5'}} as any);
});

test('loads the list and filters it on the client', async () => {
    const result = setup({pref: pref({healthFilter: ['Healthy']}), appNamespace: '', hydratorEnabled: false});
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows.map(row => row.metadata.name)).toEqual(['a']);
    expect(result.current.stats.total).toBe(1);
    // The health group ignores its own filter.
    expect(result.current.facets.totalByHealthStatus).toEqual({Healthy: 1, Degraded: 1});
    expect(result.current.searchSuggestions).toHaveLength(2);
    expect(result.current.noApplications).toBe(false);
});

test('applies watch events from the version the list was read at', async () => {
    const result = setup({pref: pref(), appNamespace: '', hydratorEnabled: false});
    await waitFor(() => expect(services.applications.watch).toHaveBeenCalled());
    expect(jest.mocked(services.applications.watch).mock.calls[0][1]).toEqual({projects: [], appNamespace: '', resourceVersion: '5'});

    act(() => {
        watch.next({type: 'ADDED', application: app('c')});
        watch.next({type: 'MODIFIED', application: app('b', 'Healthy')});
        watch.next({type: 'DELETED', application: app('a')});
    });

    await waitFor(() => expect(result.current.rows.map(row => row.metadata.name)).toEqual(['c', 'b']));
    expect(result.current.stats.totalByHealthStatus).toEqual({Healthy: 2});
});

test('ignores watch events from other namespaces when the list is scoped to one', async () => {
    const result = setup({pref: pref(), appNamespace: 'argocd', hydratorEnabled: false});
    await waitFor(() => expect(services.applications.watch).toHaveBeenCalled());
    expect(jest.mocked(services.applications.watch).mock.calls[0][1]).toEqual({projects: [], appNamespace: 'argocd', resourceVersion: '5'});

    const elsewhere = {...app('c'), metadata: {name: 'c', namespace: 'other'}} as Application;
    act(() => {
        watch.next({type: 'ADDED', application: elsewhere});
        watch.next({type: 'ADDED', application: app('d')});
    });

    await waitFor(() => expect(result.current.rows.map(row => row.metadata.name)).toEqual(['d', 'a', 'b']));
});

test('neither lists nor watches when favorites only is on without favorites', async () => {
    const result = setup({pref: pref({showFavorites: true, favoritesAppList: []}), appNamespace: '', hydratorEnabled: false});
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([]);
    expect(result.current.noApplications).toBe(false);
    expect(services.applications.list).not.toHaveBeenCalled();
    expect(services.applications.watch).not.toHaveBeenCalled();
});

test('marks an application as refreshing and requests a refresh', async () => {
    const result = setup({pref: pref(), appNamespace: '', hydratorEnabled: false});
    await waitFor(() => expect(result.current.loading).toBe(false));
    const before = result.current.rows[0];

    act(() => result.current.markRefreshing('a', 'argocd'));

    await waitFor(() => expect(result.current.rows[0].metadata.annotations?.[AnnotationRefreshKey]).toBe('refreshing'));
    // The cached object is replaced, not mutated.
    expect(before.metadata.annotations).toBeUndefined();
    expect(services.applications.get).toHaveBeenCalledWith('a', 'argocd', 'application', 'normal');
});

test('reports a failed load and retries it', async () => {
    jest.mocked(services.applications.list).mockRejectedValueOnce(new Error('boom'));
    const result = setup({pref: pref(), appNamespace: '', hydratorEnabled: false});
    await waitFor(() => expect(result.current.failed).toBe(true));

    act(() => {
        result.current.retry();
    });

    await waitFor(() => expect(result.current.rows).toHaveLength(2));
    expect(result.current.failed).toBe(false);
});

const setHidden = (hidden: boolean) => {
    Object.defineProperty(document, 'hidden', {configurable: true, get: () => hidden});
    document.dispatchEvent(new Event('visibilitychange'));
};

test('loads only while visible, and reloads before watching again after being hidden', async () => {
    setHidden(true);
    try {
        const result = setup({pref: pref(), appNamespace: '', hydratorEnabled: false});
        await act(async () => {});
        expect(services.applications.list).not.toHaveBeenCalled();

        setHidden(false);
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(1), {timeout: 2000});
        expect(services.applications.list).toHaveBeenCalledTimes(1);
        expect(result.current.rows).toHaveLength(2);

        jest.mocked(services.applications.list).mockResolvedValue({items: [app('a')], metadata: {resourceVersion: '9'}} as any);
        setHidden(true);
        await waitFor(() => expect(watch.observed).toBe(false), {timeout: 2000});

        setHidden(false);
        await waitFor(() => expect(result.current.rows).toHaveLength(1), {timeout: 2000});
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(2));
        // The second watch starts from the reloaded list, never resuming the stale version.
        expect(jest.mocked(services.applications.watch).mock.calls[1][1]).toEqual({projects: [], appNamespace: '', resourceVersion: '9'});
    } finally {
        setHidden(false);
    }
});

test('does not resume the watch from the stale list when the reload after being hidden fails', async () => {
    try {
        const result = setup({pref: pref(), appNamespace: '', hydratorEnabled: false});
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(1));

        setHidden(true);
        await waitFor(() => expect(watch.observed).toBe(false), {timeout: 2000});

        jest.mocked(services.applications.list).mockRejectedValueOnce(new Error('boom'));
        setHidden(false);
        await waitFor(() => expect(services.applications.list).toHaveBeenCalledTimes(2), {timeout: 2000});
        await act(async () => {});
        expect(services.applications.watch).toHaveBeenCalledTimes(1);
        expect(result.current.failed).toBe(true);

        jest.mocked(services.applications.list).mockResolvedValue({items: [app('a')], metadata: {resourceVersion: '9'}} as any);
        act(() => {
            result.current.retry();
        });
        await waitFor(() => expect(services.applications.watch).toHaveBeenCalledTimes(2));
        expect(jest.mocked(services.applications.watch).mock.calls[1][1]).toEqual({projects: [], appNamespace: '', resourceVersion: '9'});
        expect(result.current.failed).toBe(false);
        expect(result.current.rows).toHaveLength(1);
    } finally {
        setHidden(false);
    }
});

test('AppsListPaginate pages through the rows on the client', () => {
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(of({pageSizes: {'applications-list': 2}} as any));
    const list = {rows: [app('a'), app('b'), app('c')]} as AppsListSourceResult;
    render(
        <AppsListPaginate list={list} page={1} preferencesKey='applications-list' onPageChange={jest.fn()}>
            {rows => <p>{rows.map(row => row.metadata.name).join(',')}</p>}
        </AppsListPaginate>
    );
    expect(screen.getByText('c')).toBeInTheDocument();
});
