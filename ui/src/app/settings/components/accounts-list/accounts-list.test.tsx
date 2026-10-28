import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {AccountsList} from './accounts-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    FlexTopBar: (): null => null,
    Paginate: (props: {data: any[]; children: (data: any[]) => React.ReactNode}) => props.children(props.data)
}));

const mockSidebarTarget = {current: document.createElement('div')};
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => mockSidebarTarget}));

jest.mock('../../../shared/services', () => ({
    services: {accounts: {list: jest.fn()}}
}));

const renderList = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {}, history: {location: {search: ''}}} as any}>
                <MemoryRouter>
                    <AccountsList />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('AccountsList', () => {
    beforeEach(() => jest.clearAllMocks());

    it('renders the loaded accounts', async () => {
        jest.mocked(services.accounts.list).mockResolvedValue([{name: 'alice', enabled: true, capabilities: ['login']}] as any);
        renderList();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('alice')).toBeInTheDocument();
    });

    it('offers a retry when the accounts fail to load', async () => {
        jest.mocked(services.accounts.list).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce([]);
        renderList();

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('No accounts yet')).toBeInTheDocument();
        expect(services.accounts.list).toHaveBeenCalledTimes(2);
    });
});
