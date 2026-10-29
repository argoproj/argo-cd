import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';
import {NotificationType} from 'argo-ui';
import {of} from 'rxjs';

import {Provider} from '../../../shared/context';
import {newQueryClient} from '../../../shared/query-client';
import {services} from '../../../shared/services';
import {ReposList} from './repos-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    FlexTopBar: (props: {toolbar: {actionMenu: {items: {title: string; action: () => void}[]}}}) =>
        props.toolbar.actionMenu.items.map(item => (
            <button key={item.title} onClick={item.action}>
                {item.title}
            </button>
        )),
    Paginate: (props: {data: any[]; children: (data: any[]) => React.ReactNode}) => props.children(props.data),
    ActionMenu: (props: {items: {title: string; action: () => void}[]}) =>
        props.items.map(item => (
            <button key={item.title} onClick={item.action}>
                {item.title}
            </button>
        ))
}));

jest.mock('../repo-details/repo-details', () => ({RepoDetails: (): null => null}));

const mockSidebarTarget = {current: document.createElement('div')};
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => mockSidebarTarget}));

jest.mock('../../../shared/services', () => ({
    services: {
        authService: {settings: jest.fn()},
        repos: {list: jest.fn(), listNoCache: jest.fn(() => Promise.resolve([])), listWrite: jest.fn(), delete: jest.fn(() => Promise.resolve()), deleteWrite: jest.fn(() => Promise.resolve())},
        repocreds: {list: jest.fn(() => Promise.resolve([])), listWrite: jest.fn(() => Promise.resolve([]))},
        projects: {list: jest.fn(() => Promise.resolve([]))},
        viewPreferences: {getPreferences: () => of({hideSidebar: false})}
    }
}));

const repo = (name: string) => ({name, repo: `https://github.com/argoproj/${name}.git`, type: 'git', connectionState: {status: 'Successful'}});

const renderList = (show = jest.fn(), client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <Provider
                value={{notifications: {show}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                <MemoryRouter initialEntries={['/settings/repos']}>
                    <Route path='/settings/repos' component={ReposList} />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('ReposList', () => {
    beforeEach(() => jest.clearAllMocks());

    it('skips write repositories when the hydrator is disabled', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({hydratorEnabled: false} as any);
        jest.mocked(services.repos.list).mockResolvedValue([repo('read')] as any);
        renderList();

        expect(await screen.findByText('read')).toBeInTheDocument();
        expect(services.repos.listWrite).not.toHaveBeenCalled();
        expect(services.repocreds.listWrite).not.toHaveBeenCalled();
    });

    it('refetches read and write repositories after disconnecting one', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({hydratorEnabled: true} as any);
        jest.mocked(services.repos.list).mockResolvedValue([repo('read')] as any);
        jest.mocked(services.repos.listWrite)
            .mockResolvedValueOnce([repo('write')] as any)
            .mockResolvedValueOnce([]);
        renderList();

        const writeRow = (await screen.findByText('write')).closest('.argo-table-list__row') as HTMLElement;
        fireEvent.click(within(writeRow).getByText('Disconnect'));

        await waitFor(() => expect(services.repos.deleteWrite).toHaveBeenCalledWith('https://github.com/argoproj/write.git', ''));
        await waitFor(() => expect(screen.queryByText('write')).not.toBeInTheDocument());
        expect(screen.getByText('read')).toBeInTheDocument();
        expect(services.repos.list).toHaveBeenCalledTimes(2);
        expect(services.repos.listWrite).toHaveBeenCalledTimes(2);
    });

    it('keeps showing repositories when a refetch fails', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({hydratorEnabled: false} as any);
        jest.mocked(services.repos.list)
            .mockResolvedValueOnce([repo('read')] as any)
            .mockRejectedValueOnce(new Error('boom'));
        renderList();

        fireEvent.click(await screen.findByText('Disconnect'));

        await waitFor(() => expect(services.repos.list).toHaveBeenCalledTimes(2));
        expect(screen.getByText('read')).toBeInTheDocument();
        expect(screen.queryByText('try again')).not.toBeInTheDocument();
    });

    it('only reports a refresh as successful when the refetch succeeds', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({hydratorEnabled: false} as any);
        jest.mocked(services.repos.list)
            .mockResolvedValueOnce([repo('read')] as any)
            .mockResolvedValueOnce([repo('read')] as any)
            .mockRejectedValueOnce(new Error('boom'));
        const show = jest.fn();
        renderList(show, newQueryClient({show} as any));
        await screen.findByText('read');

        fireEvent.click(screen.getByText('Refresh list'));
        await waitFor(() => expect(show).toHaveBeenCalledWith(expect.objectContaining({content: 'Successfully reloaded list of repositories'})));

        show.mockClear();
        fireEvent.click(screen.getByText('Refresh list'));
        await waitFor(() => expect(services.repos.list).toHaveBeenCalledTimes(3));
        await new Promise(resolve => setTimeout(resolve, 0));
        // Only the query client's load-failure notification.
        expect(show).toHaveBeenCalledTimes(1);
        expect(show).toHaveBeenCalledWith(expect.objectContaining({type: NotificationType.Error}));
    });
});
