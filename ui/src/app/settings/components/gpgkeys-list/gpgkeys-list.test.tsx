import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {GpgKeysList} from './gpgkeys-list';

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
    services: {gpgkeys: {list: jest.fn(), create: jest.fn(), delete: jest.fn(() => Promise.resolve())}}
}));

const key = {keyID: 'ABCDEF', subType: 'rsa4096', owner: 'Jane <jane@example.com>'};

const renderList = (search = '') =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider
                value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                <MemoryRouter initialEntries={[`/settings/gpgkeys${search}`]}>
                    <Route path='/settings/gpgkeys' component={GpgKeysList} />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('GpgKeysList', () => {
    beforeEach(() => jest.clearAllMocks());

    it('refetches the keys after removing one', async () => {
        jest.mocked(services.gpgkeys.list).mockResolvedValueOnce([key] as any).mockResolvedValueOnce([]);
        renderList();

        fireEvent.click(await screen.findByText('Remove'));

        await waitFor(() => expect(services.gpgkeys.delete).toHaveBeenCalledWith('ABCDEF'));
        expect(await screen.findByText('No GnuPG public keys currently configured')).toBeInTheDocument();
        expect(services.gpgkeys.list).toHaveBeenCalledTimes(2);
    });

    it('disables Create while the key is being added', async () => {
        jest.mocked(services.gpgkeys.list).mockResolvedValue([]);
        jest.mocked(services.gpgkeys.create).mockReturnValue(new Promise(() => {}));
        renderList('?addGnuPGPublicKey=true');

        fireEvent.change(await screen.findByRole('textbox'), {
            target: {value: '-----BEGIN PGP PUBLIC KEY BLOCK-----\nkey\n-----END PGP PUBLIC KEY BLOCK-----'}
        });
        const create = screen.getByRole('button', {name: 'Create'});
        fireEvent.click(create);

        await waitFor(() => expect(create).toBeDisabled());
        fireEvent.click(create);
        expect(services.gpgkeys.create).toHaveBeenCalledTimes(1);
    });
});
