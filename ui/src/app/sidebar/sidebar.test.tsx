import {QueryClientProvider} from '@tanstack/react-query';
import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import * as React from 'react';
import {Route, Router, Switch} from 'react-router';

import {Provider} from '../shared/context';
import {newQueryClient} from '../shared/query-client';
import {services, ViewPreferences} from '../shared/services';
import {Sidebar} from './sidebar';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: React.PropsWithChildren) => children}));
jest.mock('../shared/services', () => ({services: {version: {version: jest.fn()}, viewPreferences: {updatePreferences: jest.fn()}}}));

const navItems = [
    {path: '/applications', title: 'Applications', iconClassName: ''},
    {path: '/applicationsets', title: 'ApplicationSets', iconClassName: ''},
    {path: '/resources', title: 'Resources', iconClassName: ''},
    {path: '/settings', title: 'Settings', iconClassName: ''},
    {path: '/user-info', title: 'User Info', iconClassName: ''},
    {path: '/help', title: 'Documentation', iconClassName: ''}
];

function renderNavigation(client: ReturnType<typeof newQueryClient>) {
    const history = createMemoryHistory({initialEntries: ['/applications']});
    const view = render(
        <QueryClientProvider client={client}>
            <Provider value={{history, baseHref: '/'} as any}>
                <Router history={history}>
                    <Switch>
                        {navItems.map(item => (
                            <Route key={item.path} path={item.path}>
                                <Sidebar navItems={navItems} pref={{hideSidebar: false} as ViewPreferences} onVersionClick={jest.fn()} />
                            </Route>
                        ))}
                    </Switch>
                </Router>
            </Provider>
        </QueryClientProvider>
    );
    return {...view, history};
}

describe('sidebar version during navigation', () => {
    let notifications: {show: jest.Mock};
    let client: ReturnType<typeof newQueryClient>;

    beforeEach(() => {
        jest.resetAllMocks();
        notifications = {show: jest.fn()};
        client = newQueryClient(notifications as any);
    });

    afterEach(() => {
        cleanup();
        client.clear();
    });

    it('keeps the resolved version across top-level remounts and a delayed refresh', async () => {
        let resolveRefresh: (value: any) => void;
        jest.mocked(services.version.version)
            .mockResolvedValueOnce({Version: 'v3.7.0'} as any)
            .mockImplementation(() => new Promise(resolve => (resolveRefresh = resolve)));
        const {history} = renderNavigation(client);

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('v3.7.0')).toBeInTheDocument();
        for (const item of [...navItems.slice(1), navItems[0]]) {
            fireEvent.click(screen.getByRole('link', {name: item.title}));
            expect(history.location.pathname).toBe(item.path);
            expect(screen.getByText('v3.7.0')).toBeInTheDocument();
            expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
        }
        expect(services.version.version).toHaveBeenCalledTimes(2);
        await act(async () => resolveRefresh({Version: 'v3.7.1'}));
        expect(await screen.findByText('v3.7.1')).toBeInTheDocument();
    });

    it('keeps the last success through failed refreshes without notifications and retries on return', async () => {
        jest.mocked(services.version.version)
            .mockResolvedValueOnce({Version: 'v3.7.0'} as any)
            .mockRejectedValueOnce(new Error('unavailable'))
            .mockRejectedValueOnce(new Error('unavailable'))
            .mockResolvedValueOnce({Version: 'v3.7.1'} as any);
        renderNavigation(client);
        expect(await screen.findByText('v3.7.0')).toBeInTheDocument();

        for (const [index, name] of ['Settings', 'Resources'].entries()) {
            fireEvent.click(screen.getByRole('link', {name}));
            await waitFor(() => expect(services.version.version).toHaveBeenCalledTimes(index + 2));
            await waitFor(() => expect(client.isFetching()).toBe(0));
            expect(screen.getByText('v3.7.0')).toBeInTheDocument();
            expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
            expect(screen.queryByText('Unknown')).not.toBeInTheDocument();
            expect(notifications.show).not.toHaveBeenCalled();
        }

        fireEvent.click(screen.getByRole('link', {name: 'Applications'}));
        expect(await screen.findByText('v3.7.1')).toBeInTheDocument();
    });

    it('shows loading during a delayed retry after the initial request fails', async () => {
        let resolveRetry: (value: any) => void;
        jest.mocked(services.version.version)
            .mockRejectedValueOnce(new Error('unavailable'))
            .mockImplementationOnce(() => new Promise(resolve => (resolveRetry = resolve)));
        renderNavigation(client);
        expect(await screen.findByText('Unknown')).toBeInTheDocument();

        fireEvent.click(screen.getByRole('link', {name: 'Settings'}));
        await waitFor(() => expect(services.version.version).toHaveBeenCalledTimes(2));
        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(screen.queryByText('Unknown')).not.toBeInTheDocument();

        await act(async () => resolveRetry({Version: 'v3.7.0'}));
        expect(await screen.findByText('v3.7.0')).toBeInTheDocument();
        expect(notifications.show).not.toHaveBeenCalled();
    });
});
