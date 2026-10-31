import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {Provider} from '../../shared/context';
import {services} from '../../shared/services';
import {Help} from './help';

jest.mock('../../shared/components', () => ({Page: (props: {children?: React.ReactNode}) => props.children}));
jest.mock('../../shared/services', () => ({services: {authService: {settings: jest.fn()}, version: {version: jest.fn()}}}));

const renderHelp = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
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
});
