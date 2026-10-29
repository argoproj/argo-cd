import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {CertsList} from './certs-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    FlexTopBar: (): null => null,
    Paginate: (props: {data: any[]; children: (data: any[]) => React.ReactNode}) => props.children(props.data),
    ActionMenu: (props: {items: {title: string; action: () => void}[]}) =>
        props.items.map(item => (
            <button key={item.title} onClick={item.action}>
                {item.title}
            </button>
        ))
}));

const mockSidebarTarget = {current: document.createElement('div')};
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => mockSidebarTarget}));

jest.mock('../../../shared/services', () => ({
    services: {certs: {list: jest.fn(), delete: jest.fn(() => Promise.resolve())}}
}));

const cert = {serverName: 'github.com', certType: 'ssh', certSubType: 'ssh-ed25519', certInfo: 'SHA256:abc'};

describe('CertsList', () => {
    beforeEach(() => jest.clearAllMocks());

    it('refetches the certificates after removing one', async () => {
        jest.mocked(services.certs.list).mockResolvedValueOnce([cert] as any).mockResolvedValueOnce([]);
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <Provider
                    value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                    <MemoryRouter initialEntries={['/settings/certs']}>
                        <Route path='/settings/certs' component={CertsList} />
                    </MemoryRouter>
                </Provider>
            </QueryClientProvider>
        );

        fireEvent.click(await screen.findByText('Remove'));

        await waitFor(() => expect(services.certs.delete).toHaveBeenCalledWith('github.com', 'ssh', 'ssh-ed25519'));
        expect(await screen.findByText('No certificates configured')).toBeInTheDocument();
        expect(services.certs.list).toHaveBeenCalledTimes(2);
    });

    it('keeps showing the certificates when a refetch fails', async () => {
        jest.mocked(services.certs.list).mockResolvedValueOnce([cert] as any).mockRejectedValueOnce(new Error('boom'));
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <Provider
                    value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                    <MemoryRouter initialEntries={['/settings/certs']}>
                        <Route path='/settings/certs' component={CertsList} />
                    </MemoryRouter>
                </Provider>
            </QueryClientProvider>
        );

        fireEvent.click(await screen.findByText('Remove'));

        await waitFor(() => expect(services.certs.list).toHaveBeenCalledTimes(2));
        expect(screen.getByText('github.com')).toBeInTheDocument();
        expect(screen.queryByText('try again')).not.toBeInTheDocument();
    });

    it('notifies when removing a certificate fails', async () => {
        jest.mocked(services.certs.list).mockResolvedValue([cert] as any);
        jest.mocked(services.certs.delete).mockRejectedValueOnce(new Error('denied'));
        const show = jest.fn();
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <Provider value={{notifications: {show}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                    <MemoryRouter initialEntries={['/settings/certs']}>
                        <Route path='/settings/certs' component={CertsList} />
                    </MemoryRouter>
                </Provider>
            </QueryClientProvider>
        );

        fireEvent.click(await screen.findByText('Remove'));

        await waitFor(() => expect(show).toHaveBeenCalledTimes(1));
        expect(services.certs.list).toHaveBeenCalledTimes(1);
    });
});
