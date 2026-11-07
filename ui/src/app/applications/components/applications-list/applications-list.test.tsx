import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {NavigationManager} from 'argo-ui';
import {createMemoryHistory, MemoryHistory} from 'history';
import * as React from 'react';
import {BehaviorSubject} from 'rxjs';

import {Context} from '../../../shared/context';
import {Application} from '../../../shared/models';
import {services} from '../../../shared/services';
import {useApplicationsList} from './applications-list-client';
import {AppsListSourceResult} from './applications-list-source';

// Renders the page with the real navigation and URL handling, and stubs the parts around it: the data
// source, the table and the panels.

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {getPreferences: jest.fn(), updatePreferences: jest.fn()},
        clusters: {list: jest.fn(() => Promise.resolve([]))},
        applications: {get: jest.fn()}
    },
    AppsListViewKey: {List: 'list', Tiles: 'tiles', Summary: 'summary'},
    AppsListPreferences: {clearFilters: jest.fn()}
}));

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: ({children}: {children: React.ReactNode}) => <>{children}</>,
    FlexTopBar: () => null
}));

jest.mock('../../../sidebar/sidebar', () => {
    const target = {current: document.createElement('div')};
    return {useSidebarTarget: () => target};
});

jest.mock('./applications-list-client', () => ({
    useApplicationsList: jest.fn(),
    AppsListPaginate: ({list, children}: {list: AppsListSourceResult; children: (rows: Application[], virtual: boolean) => React.ReactNode}) => <>{children(list.rows, false)}</>
}));

jest.mock('./applications-filter', () => ({ApplicationsFilter: () => null}));
jest.mock('./applications-status-bar', () => ({AppsStatusBar: () => null}));
jest.mock('../applications-sync-panel/applications-sync-panel', () => ({ApplicationsSyncPanel: () => null}));
jest.mock('../applications-refresh-panel/applications-refresh-panel', () => ({ApplicationsRefreshPanel: () => null}));

// Both views render the same stub, so a test can sync from either.
const mockSyncButtons = ({applications, syncApplication}: {applications: Application[]; syncApplication: (name: string, namespace: string) => void}) => (
    <>
        {applications.map(app => (
            <button key={app.metadata.name} onClick={() => syncApplication(app.metadata.name, app.metadata.namespace)}>
                Sync {app.metadata.name}
            </button>
        ))}
    </>
);
jest.mock('./applications-table', () => ({ApplicationsTable: mockSyncButtons}));
jest.mock('./applications-tiles', () => ({ApplicationTiles: mockSyncButtons}));

jest.mock('../application-sync-panel/application-sync-panel', () => ({
    ApplicationSyncPanel: ({application, hide}: {application: Application; hide: () => void}) =>
        application && (
            <div>
                Syncing {application.metadata.name}
                <button onClick={hide}>Close</button>
            </div>
        )
}));

import {ApplicationsList} from './applications-list';

const app = (name: string, namespace: string) => ({metadata: {name, namespace}, spec: {}, status: {}}) as unknown as Application;

const list = (rows: Application[]) =>
    ({
        loading: false,
        failed: false,
        retry: jest.fn(),
        rows,
        facets: {},
        stats: {total: rows.length},
        noApplications: false,
        searchSuggestions: rows,
        markRefreshing: jest.fn()
    }) as unknown as AppsListSourceResult;

const renderAt = (url: string) => {
    const history = createMemoryHistory({initialEntries: [url]});
    const apis = {navigation: new NavigationManager(history), notifications: {show: jest.fn()}, popup: {confirm: jest.fn()}, baseHref: '/'};
    const props = {match: {url: '/applications'}} as any;
    render(
        <Context.Provider value={{...apis, history} as any}>
            <ApplicationsList {...props} />
        </Context.Provider>
    );
    return history;
};

const params = (history: MemoryHistory) => new URLSearchParams(history.location.search);
const listedNamespace = () => jest.mocked(useApplicationsList).mock.lastCall[0].appNamespace;

beforeEach(() => {
    jest.clearAllMocks();
    jest.mocked(services.viewPreferences.getPreferences).mockReturnValue(new BehaviorSubject({appList: {view: 'list'}} as any));
    jest.mocked(useApplicationsList).mockReturnValue(list([app('guestbook', 'team-a')]));
    jest.mocked(services.applications.get).mockImplementation((name, namespace) => Promise.resolve(app(name, namespace)));
});

test.each(['list', 'tiles'])('syncing an application from the %s view opens its panel without scoping the list to its namespace', async view => {
    const history = renderAt(`/applications?view=${view}`);
    expect(listedNamespace()).toBeNull();

    fireEvent.click(await screen.findByText('Sync guestbook'));

    expect(await screen.findByText('Syncing guestbook')).toBeInTheDocument();
    expect(services.applications.get).toHaveBeenCalledWith('guestbook', 'team-a', 'application');
    expect(params(history).get('appNamespace')).toBeNull();
    expect(listedNamespace()).toBeNull();

    fireEvent.click(screen.getByText('Close'));

    await waitFor(() => expect(screen.queryByText('Syncing guestbook')).not.toBeInTheDocument());
    expect(params(history).get('syncApp')).toBeNull();
    expect(params(history).get('syncAppNamespace')).toBeNull();
});

test('keeps the list scoped to the namespace in the URL across syncing an application', async () => {
    const history = renderAt('/applications?appNamespace=team-a');
    expect(listedNamespace()).toBe('team-a');

    fireEvent.click(await screen.findByText('Sync guestbook'));
    await screen.findByText('Syncing guestbook');
    fireEvent.click(screen.getByText('Close'));
    await act(async () => {});

    expect(params(history).get('appNamespace')).toBe('team-a');
    expect(listedNamespace()).toBe('team-a');
});
