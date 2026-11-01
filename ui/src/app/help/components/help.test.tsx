import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';

import {Provider} from '../../shared/context';
import {services} from '../../shared/services';
import {Help} from './help';

jest.mock('../../shared/components', () => ({Page: (props: {children?: React.ReactNode}) => props.children}));
jest.mock('../../shared/services', () => ({services: {authService: {settings: jest.fn()}, version: {version: jest.fn()}}}));

const renderHelp = (client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <Provider value={{} as any}>
                <Help />
            </Provider>
        </QueryClientProvider>
    );

describe('Help', () => {
    beforeEach(() => jest.clearAllMocks());

    it('shows downloads with the server architecture', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({help: {binaryUrls: {'argocd-darwin-arm64': '/darwin'}}} as any);
        jest.mocked(services.version.version).mockResolvedValue({Platform: 'linux/amd64'} as any);
        renderHelp();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('Linux (amd64)')).toBeInTheDocument();
        expect(screen.getByText(/darwin/)).toBeInTheDocument();
    });

    it('retries failed settings without requiring version data', async () => {
        jest.mocked(services.authService.settings)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({help: {binaryUrls: {}}} as any);
        jest.mocked(services.version.version).mockRejectedValue(new Error('unavailable'));
        renderHelp();

        fireEvent.click(await screen.findByText('try again'));
        expect(await screen.findByText('New to Argo CD?')).toBeInTheDocument();
        expect(services.authService.settings).toHaveBeenCalledTimes(2);
    });

    it('keeps cached downloads visible and offers retry after a refetch fails', async () => {
        jest.mocked(services.authService.settings)
            .mockResolvedValueOnce({help: {binaryUrls: {'argocd-darwin-arm64': '/old'}}} as any)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({help: {binaryUrls: {'argocd-darwin-arm64': '/new'}}} as any);
        jest.mocked(services.version.version).mockResolvedValue({Platform: 'linux/amd64'} as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        renderHelp(client);
        expect(await screen.findByRole('link', {name: /darwin/})).toHaveAttribute('href', '/old');

        await act(() => client.invalidateQueries({queryKey: ['help', 'settings']}));
        expect(await screen.findByText('try again')).toBeInTheDocument();
        expect(screen.getByRole('link', {name: /darwin/})).toHaveAttribute('href', '/old');

        fireEvent.click(screen.getByText('try again'));
        await waitFor(() => expect(screen.getByRole('link', {name: /darwin/})).toHaveAttribute('href', '/new'));
    });
});
