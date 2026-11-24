import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {NavigationManager} from 'argo-ui';
import {createMemoryHistory, MemoryHistory} from 'history';
import * as React from 'react';
import {Subject} from 'rxjs';

import {Context} from '../../../shared/context';
import {Resource, ViewPreferences} from '../../../shared/models';
import {services} from '../../../shared/services';
import {RESOURCES_LIST_SORT_KEY} from './resources-sort';

// Renders the page with the real view preferences service and URL handling, and stubs the table, sidebar
// filter and panels to record what they are given.

jest.mock('../../../shared/services', () => {
    const actual = jest.requireActual('../../../shared/services');
    const {ViewPreferencesService} = jest.requireActual('../../../shared/services/view-preferences-service');
    return {
        ...actual,
        services: {
            viewPreferences: new ViewPreferencesService(),
            applications: {list: jest.fn(), watch: jest.fn()},
            clusters: {list: jest.fn(() => Promise.resolve([]))}
        }
    };
});

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: ({children}: {children: React.ReactNode}) => <>{children}</>,
    FlexTopBar: (): null => null,
    Query: (): null => null,
    Paginate: ({data, emptyState, children}: {data: Resource[]; emptyState: () => React.ReactNode; children: (data: Resource[], virtual: boolean) => React.ReactNode}) => (
        <>{data.length === 0 ? emptyState() : children(data, false)}</>
    )
}));

jest.mock('../../../sidebar/sidebar', () => {
    const target = {current: document.body.appendChild(document.createElement('div'))};
    return {useSidebarTarget: () => target};
});

let tableRenders = 0;
jest.mock('./resources-table', () => ({
    ResourcesTable: ({resources}: {resources: Resource[]}) => {
        tableRenders++;
        return <ul>{resources.map(r => <li key={r.name}>{`${r.kind}/${r.name}`}</li>)}</ul>;
    }
}));

jest.mock('./resources-filter', () => ({
    ...jest.requireActual('./resources-filter'),
    ResourcesFilter: ({collapsed, pref}: {collapsed: boolean; pref: {kindFilter: string[]}}) => (
        <div data-testid='filter'>{`collapsed=${!!collapsed} kinds=${pref.kindFilter.join(',')}`}</div>
    )
}));
jest.mock('./resources-summary', () => ({ResourcesSummary: (): null => null}));
jest.mock('./resources-status-bar', () => ({ResourcesStatusBar: (): null => null}));
jest.mock('./resources-details-panel', () => ({ResourcesDetailsPanel: (): null => null}));

import {ResourcesList} from './resources-list';

const app = {
    metadata: {name: 'guestbook', namespace: 'argocd'},
    spec: {project: 'default', destination: {server: 'https://kubernetes.default.svc'}},
    status: {
        resources: [
            {kind: 'Service', name: 'b-svc', namespace: 'default', version: 'v1', status: 'Synced'},
            {kind: 'Deployment', name: 'a-deploy', namespace: 'default', group: 'apps', version: 'v1', status: 'Synced'}
        ]
    }
};

let defaults: ViewPreferences;
beforeAll(() => {
    window.localStorage.clear();
    services.viewPreferences.init();
    defaults = JSON.parse(JSON.stringify(services.viewPreferences.getCurrentPreferences()));
});

beforeEach(() => {
    tableRenders = 0;
    jest.mocked(services.applications.list).mockResolvedValue({items: [app], metadata: {resourceVersion: '1'}} as any);
    jest.mocked(services.applications.watch).mockReturnValue(new Subject());
    // The service keeps its state across tests; put back the sections these tests change.
    services.viewPreferences.updatePreferences({
        resourcesList: JSON.parse(JSON.stringify(defaults.resourcesList)),
        hideSidebar: false,
        theme: defaults.theme,
        sortOptions: {},
        sortDirections: {}
    });
});

const renderAt = (url: string) => {
    const history = createMemoryHistory({initialEntries: [url]});
    const apis = {navigation: new NavigationManager(history), notifications: {show: jest.fn()}, popup: {confirm: jest.fn()}, baseHref: '/'};
    const props = {location: history.location, match: {url: '/resources'}} as any;
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Context.Provider value={{...apis, history} as any}>
                <ResourcesList {...props} />
            </Context.Provider>
        </QueryClientProvider>
    );
    return history;
};

const rows = () => screen.getAllByRole('listitem').map(item => item.textContent);

test('URL filters override the stored filters and follow navigation', async () => {
    services.viewPreferences.updatePreferences({resourcesList: {...services.viewPreferences.getCurrentPreferences().resourcesList, kindFilter: ['Service']}});
    const history = renderAt('/resources?kind=Deployment');

    expect(await screen.findByText('Deployment/a-deploy')).toBeInTheDocument();
    expect(screen.queryByText('Service/b-svc')).not.toBeInTheDocument();

    act(() => history.push('/resources?kind=Service'));
    expect(await screen.findByText('Service/b-svc')).toBeInTheDocument();
    expect(screen.getByTestId('filter')).toHaveTextContent('kinds=Service');
});

test('sorts by the stored sort preference', async () => {
    renderAt('/resources');
    await screen.findByText('Deployment/a-deploy');
    expect(rows()).toEqual(['Deployment/a-deploy', 'Service/b-svc']);

    act(() => services.viewPreferences.updatePreferences({sortDirections: {[RESOURCES_LIST_SORT_KEY]: 'desc'}}));
    expect(rows()).toEqual(['Service/b-svc', 'Deployment/a-deploy']);
});

test('the sidebar filter follows the sidebar preference', async () => {
    renderAt('/resources');
    expect(await screen.findByTestId('filter')).toHaveTextContent('collapsed=false');

    act(() => services.viewPreferences.updatePreferences({hideSidebar: true}));
    expect(screen.getByTestId('filter')).toHaveTextContent('collapsed=true');
});

test('re-renders the list for its own preferences only', async () => {
    renderAt('/resources');
    await screen.findByText('Deployment/a-deploy');
    const settled = tableRenders;

    act(() => services.viewPreferences.updatePreferences({theme: 'dark'}));
    act(() => services.viewPreferences.updatePreferences({hideSidebar: true}));
    expect(tableRenders).toBe(settled);

    act(() => services.viewPreferences.updatePreferences({resourcesList: {...services.viewPreferences.getCurrentPreferences().resourcesList, statusBarView: {showHealthStatusBar: false}}}));
    expect(tableRenders).toBeGreaterThan(settled);
});

test('clear filters resets the stored filters and the URL', async () => {
    const history: MemoryHistory = renderAt('/resources?kind=ConfigMap');
    expect(await screen.findByTestId('filter')).toHaveTextContent('kinds=ConfigMap');
    fireEvent.click(await screen.findByText('clear filters'));

    await waitFor(() => expect(screen.getByText('Deployment/a-deploy')).toBeInTheDocument());
    expect(services.viewPreferences.getCurrentPreferences().resourcesList.kindFilter).toEqual([]);
    expect(new URLSearchParams(history.location.search).get('kind')).toBeFalsy();
});
