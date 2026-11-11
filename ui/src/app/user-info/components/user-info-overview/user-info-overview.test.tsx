import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';

import {Provider} from '../../../shared/context';
import {userQueries} from '../../../shared/queries';
import {newQueryClient} from '../../../shared/query-client';
import {services} from '../../../shared/services';
import {UserInfoOverview} from './user-info-overview';

jest.mock('../../../shared/components', () => ({
    AuthOption: () => null,
    ErrorNotification: () => null,
    Page: (props: {children?: React.ReactNode}) => props.children,
    Spinner: () => null
}));
jest.mock('../../../shared/services', () => ({services: {users: {get: jest.fn()}, accounts: {changePassword: jest.fn()}}}));

const renderOverview = (client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <Provider value={{history: {location: {pathname: '/user-info', search: ''}, push: jest.fn()}} as any}>
                <UserInfoOverview />
            </Provider>
        </QueryClientProvider>
    );

describe('UserInfoOverview', () => {
    beforeEach(() => jest.clearAllMocks());

    it('shows the logged in user', async () => {
        jest.mocked(services.users.get).mockResolvedValue({loggedIn: true, username: 'alice', iss: 'argocd', groups: ['admins']});
        renderOverview();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('Username: alice')).toBeInTheDocument();
        expect(screen.getByText('Issuer: argocd')).toBeInTheDocument();
        expect(screen.getByText('admins')).toBeInTheDocument();
    });

    it('shows when the user is not logged in', async () => {
        jest.mocked(services.users.get).mockResolvedValue({loggedIn: false, username: '', iss: '', groups: []});
        renderOverview();

        expect(await screen.findByText('You are not logged in')).toBeInTheDocument();
    });

    it('does not show the previous user after the page is left', async () => {
        jest.mocked(services.users.get)
            .mockResolvedValueOnce({loggedIn: true, username: 'alice', iss: 'argocd', groups: []})
            .mockResolvedValueOnce({loggedIn: true, username: 'bob', iss: 'argocd', groups: []});
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        const first = renderOverview(client);
        expect(await screen.findByText('Username: alice')).toBeInTheDocument();
        first.unmount();

        await waitFor(() => expect(client.getQueryData(userQueries.info().queryKey)).toBeUndefined());
        renderOverview(client);
        expect(screen.queryByText('Username: alice')).not.toBeInTheDocument();
        expect(await screen.findByText('Username: bob')).toBeInTheDocument();
    });

    it('offers retry after a failed load', async () => {
        jest.mocked(services.users.get)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({loggedIn: true, username: 'alice', iss: 'argocd', groups: []});
        const show = jest.fn();
        renderOverview(newQueryClient({show} as any));

        fireEvent.click(await screen.findByText('try again'));
        expect(await screen.findByText('Username: alice')).toBeInTheDocument();
        expect(show).toHaveBeenCalledTimes(1);
        expect(services.users.get).toHaveBeenCalledTimes(2);
    });
});
