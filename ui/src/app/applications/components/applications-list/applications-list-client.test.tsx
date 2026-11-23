import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, render, renderHook, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import * as React from 'react';
import {of, Subject} from 'rxjs';

import {AuthSettingsCtx} from '../../../shared/context';
import {AnnotationRefreshKey, Application, ApplicationWatchEvent, AuthSettings} from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import {
    AppsListPaginate,
    clampAppsListPage,
    clusterFilterToServerValue,
    requiresClientSideAppFilters,
    resolveApplicationPageSize,
    sortTitleToServer,
    useApplicationsList
} from './applications-list-client';
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
    jest.spyOn(services.viewPreferences, 'init').mockImplementation(() => undefined);
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(of({pageSizes: {}, sortOptions: {}} as any));
    jest.spyOn(services.viewPreferences, 'getCurrentPreferences').mockReturnValue({pageSizes: {}, sortOptions: {}} as any);
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

test('resolveApplicationPageSize maps all/-1 to the server default', () => {
    expect(resolveApplicationPageSize(-1, 50)).toBe(50);
    expect(resolveApplicationPageSize(undefined, 50)).toBe(50);
    expect(resolveApplicationPageSize(20, 50)).toBe(20);
});

test('sortTitleToServer maps toolbar titles', () => {
    expect(sortTitleToServer('Name')).toBe('name');
    expect(sortTitleToServer('Created At')).toBe('createdAt');
    expect(sortTitleToServer('Synchronized')).toBe('synchronizedAt');
});

test('AppsListPaginate under SSP uses stats.total for page controls', () => {
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(of({pageSizes: {'applications-list': 2}} as any));
    jest.spyOn(services.viewPreferences, 'getCurrentPreferences').mockReturnValue({pageSizes: {'applications-list': 2}} as any);
    const list = {rows: [app('a'), app('b')], stats: {total: 5}, serverPaged: true, isPlaceholderData: false} as AppsListSourceResult;
    render(
        <AuthSettingsCtx.Provider value={{serverSidePaginationEnabled: true, applicationPageSize: 50} as AuthSettings}>
            <AppsListPaginate list={list} page={0} preferencesKey='applications-list' onPageChange={jest.fn()}>
                {rows => <p>{rows.map(row => row.metadata.name).join(',')}</p>}
            </AppsListPaginate>
        </AuthSettingsCtx.Provider>
    );
    expect(screen.getByText('a,b')).toBeInTheDocument();
    expect(screen.getAllByText('Next').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Items per page: 2').length).toBeGreaterThan(0);
});

test('AppsListPaginate does not clamp page while placeholder data is shown', () => {
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(of({pageSizes: {'applications-list': 2}} as any));
    jest.spyOn(services.viewPreferences, 'getCurrentPreferences').mockReturnValue({pageSizes: {'applications-list': 2}} as any);
    const onPageChange = jest.fn();
    const list = {rows: [app('a'), app('b')], stats: {total: 2}, serverPaged: true, isPlaceholderData: true} as AppsListSourceResult;
    render(
        <AuthSettingsCtx.Provider value={{serverSidePaginationEnabled: true, applicationPageSize: 50} as AuthSettings}>
            <AppsListPaginate list={list} page={3} preferencesKey='applications-list' onPageChange={onPageChange}>
                {rows => <p>{rows.map(row => row.metadata.name).join(',')}</p>}
            </AppsListPaginate>
        </AuthSettingsCtx.Provider>
    );
    expect(onPageChange).not.toHaveBeenCalled();
});

test('clusterFilterToServerValue returns name and URL alternatives from display labels', () => {
    expect(clusterFilterToServerValue('in-cluster (https://kubernetes.default.svc)')).toEqual([
        'in-cluster',
        'https://kubernetes.default.svc'
    ]);
    expect(clusterFilterToServerValue('https://kubernetes.default.svc')).toEqual(['https://kubernetes.default.svc']);
    expect(clusterFilterToServerValue('in-cluster')).toEqual(['in-cluster']);
});

test('clampAppsListPage resets to zero when results fit on the first page', () => {
    expect(clampAppsListPage(3, 5, 10)).toBe(0);
    expect(clampAppsListPage(3, 25, 10)).toBe(2);
    expect(clampAppsListPage(9, undefined, 10)).toBe(9);
});

test('requiresClientSideAppFilters detects unsupported SSP filters', () => {
    expect(requiresClientSideAppFilters(pref(), false)).toBe(false);
    expect(requiresClientSideAppFilters(pref({targetRevisionFilter: ['HEAD']}), false)).toBe(true);
    expect(requiresClientSideAppFilters(pref({annotationsFilter: ['team=platform']}), false)).toBe(true);
    expect(requiresClientSideAppFilters(pref({operationFilter: ['Sync OK']}), false)).toBe(true);
    expect(requiresClientSideAppFilters(pref({hydrationFilter: ['Hydrated']}), false)).toBe(false);
    expect(requiresClientSideAppFilters(pref({hydrationFilter: ['Hydrated']}), true)).toBe(true);
});

test('SSP falls back to full-list client filtering for targetRevision', async () => {
    jest.mocked(services.applications.list).mockResolvedValue({
        items: [
            {...app('a'), spec: {destination: {server: 'https://kubernetes.default.svc'}, source: {targetRevision: 'HEAD'}}},
            {...app('b'), spec: {destination: {server: 'https://kubernetes.default.svc'}, source: {targetRevision: 'main'}}}
        ],
        metadata: {resourceVersion: '5'},
        stats: {total: 2}
    } as any);

    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => (
        <AuthSettingsCtx.Provider value={{serverSidePaginationEnabled: true, applicationPageSize: 50} as AuthSettings}>
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        </AuthSettingsCtx.Provider>
    );
    const {result} = renderHook(
        () => useApplicationsList({pref: pref({targetRevisionFilter: ['HEAD']}), appNamespace: '', hydratorEnabled: false}),
        {wrapper}
    );
    await waitFor(() => expect(result.current.loading).toBe(false));

    const listOpts = jest.mocked(services.applications.list).mock.calls[0][2] as any;
    expect(listOpts.limit).toBeUndefined();
    expect(listOpts.offset).toBeUndefined();
    expect(result.current.serverPaged).toBe(false);
    expect(result.current.rows.map(row => row.metadata.name)).toEqual(['a']);
    expect(result.current.stats.total).toBe(1);
});

test('SSP list sends search/sort/limit and omits search from watch', async () => {
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(
        of({pageSizes: {'applications-list': 10}, sortOptions: {'applications-list': 'Created At'}} as any)
    );
    jest.spyOn(services.viewPreferences, 'getCurrentPreferences').mockReturnValue({
        pageSizes: {'applications-list': 10},
        sortOptions: {'applications-list': 'Created At'}
    } as any);
    jest.mocked(services.applications.list).mockResolvedValue({
        items: [app('a')],
        metadata: {resourceVersion: '5'},
        stats: {total: 1, repos: ['https://example.com/repo']}
    } as any);

    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => (
        <AuthSettingsCtx.Provider value={{serverSidePaginationEnabled: true, applicationPageSize: 50} as AuthSettings}>
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        </AuthSettingsCtx.Provider>
    );
    const {result} = renderHook(
        () => useApplicationsList({pref: pref({search: 'fake', page: 1}), appNamespace: '', hydratorEnabled: false}),
        {wrapper}
    );
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.serverPaged).toBe(true);
    expect(services.applications.list).toHaveBeenCalled();
    const listOpts = jest.mocked(services.applications.list).mock.calls[0][2] as any;
    expect(listOpts.search).toBe('fake');
    expect(listOpts.limit).toBe(10);
    expect(listOpts.offset).toBe(10);
    expect(listOpts.sortBy).toBe('createdAt');

    await waitFor(() => expect(services.applications.watch).toHaveBeenCalled());
    const watchOpts = jest.mocked(services.applications.watch).mock.calls[0][2] as any;
    expect(watchOpts.search).toBeUndefined();
    expect(watchOpts.offset).toBeUndefined();
    expect(watchOpts.limit).toBeUndefined();
});

test('SSP empty search result does not show create-first empty state', async () => {
    jest.mocked(services.applications.list).mockResolvedValue({
        items: [],
        metadata: {resourceVersion: '5'},
        stats: {total: 0}
    } as any);

    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const wrapper = ({children}: {children: React.ReactNode}) => (
        <AuthSettingsCtx.Provider value={{serverSidePaginationEnabled: true, applicationPageSize: 50} as AuthSettings}>
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        </AuthSettingsCtx.Provider>
    );
    const {result} = renderHook(
        () => useApplicationsList({pref: pref({search: 'no-such-app-xyz-99999'}), appNamespace: '', hydratorEnabled: false}),
        {wrapper}
    );
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toHaveLength(0);
    expect(result.current.noApplications).toBe(false);
});
