import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {AccountDetails} from './account-details';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children
}));

jest.mock('../../../shared/services', () => ({
    services: {accounts: {get: jest.fn(), deleteToken: jest.fn(() => Promise.resolve())}}
}));

const account = (tokens: {id: string; issuedAt: number}[]) => ({name: 'alice', enabled: true, capabilities: ['apiKey'], tokens});

describe('AccountDetails', () => {
    beforeEach(() => jest.clearAllMocks());

    it('loads the account once and refetches it after deleting a token', async () => {
        jest.mocked(services.accounts.get)
            .mockResolvedValueOnce(account([{id: 'ci', issuedAt: 1}]) as any)
            .mockResolvedValueOnce(account([]) as any);
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <Provider value={{notifications: {show: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}} as any}>
                    <MemoryRouter initialEntries={['/settings/accounts/alice']}>
                        <Route path='/settings/accounts/:name' component={AccountDetails} />
                    </MemoryRouter>
                </Provider>
            </QueryClientProvider>
        );

        expect(await screen.findByText('ci')).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(1);

        fireEvent.click(screen.getByTitle('Delete'));

        await waitFor(() => expect(services.accounts.deleteToken).toHaveBeenCalledWith('alice', 'ci'));
        expect(await screen.findByText(/Account has no tokens/)).toBeInTheDocument();
        expect(services.accounts.get).toHaveBeenCalledTimes(2);
    });
});
