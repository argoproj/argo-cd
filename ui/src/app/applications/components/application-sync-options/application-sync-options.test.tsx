import * as React from 'react';
import {onlineManager, QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitForElementToBeRemoved} from '@testing-library/react';
import '@testing-library/jest-dom';

import {settingsQueries} from '../../../shared/queries';
import {newQueryClient} from '../../../shared/query-client';
import {services} from '../../../shared/services';
import {ApplicationSyncOptions} from './application-sync-options';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: (props: {children: React.ReactNode}) => props.children}));
jest.mock('../../../shared/services', () => ({services: {authService: {settings: jest.fn()}}}));

const renderOptions = (props: {options?: string[]; onChanged?: (options: string[]) => any} = {}, client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <ApplicationSyncOptions options={props.options || []} onChanged={props.onChanged || jest.fn()} id='app' />
        </QueryClientProvider>
    );

describe('ApplicationSyncOptions', () => {
    beforeEach(() => jest.clearAllMocks());

    it('offers Replace when the server allows it', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({syncWithReplaceAllowed: true} as any);
        const onChanged = jest.fn();
        renderOptions({options: ['PruneLast=true'], onChanged});

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        fireEvent.click(await screen.findByLabelText('Replace'));
        expect(onChanged).toHaveBeenCalledWith(['PruneLast=true', 'Replace=true']);
    });

    it('hides Replace when the server does not allow it', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({syncWithReplaceAllowed: false} as any);
        renderOptions();

        await waitForElementToBeRemoved(() => screen.queryByText('Loading...'));
        expect(screen.queryByLabelText('Replace')).not.toBeInTheDocument();
        expect(screen.getByLabelText('Prune Last')).toBeInTheDocument();
    });

    it('does not offer Replace from cached settings the server no longer allows', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({syncWithReplaceAllowed: false} as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        client.setQueryData(settingsQueries.get().queryKey, {syncWithReplaceAllowed: true} as any);
        renderOptions({}, client);

        expect(screen.queryByLabelText('Replace')).not.toBeInTheDocument();
        await waitForElementToBeRemoved(() => screen.queryByText('Loading...'));
        expect(screen.queryByLabelText('Replace')).not.toBeInTheDocument();
        expect(services.authService.settings).toHaveBeenCalledTimes(1);
    });

    it('refetches settings on mount even when they are not stale', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({syncWithReplaceAllowed: false} as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false, staleTime: Infinity}}});
        client.setQueryData(settingsQueries.get().queryKey, {syncWithReplaceAllowed: true} as any);
        renderOptions({}, client);

        await waitForElementToBeRemoved(() => screen.queryByText('Loading...'));
        expect(screen.queryByLabelText('Replace')).not.toBeInTheDocument();
        expect(services.authService.settings).toHaveBeenCalledTimes(1);
    });

    it('does not offer Replace from cached settings while offline', async () => {
        jest.mocked(services.authService.settings).mockRejectedValue(new Error('offline'));
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        client.setQueryData(settingsQueries.get().queryKey, {syncWithReplaceAllowed: true} as any);
        onlineManager.setOnline(false);
        try {
            renderOptions({}, client);

            expect(await screen.findByText('try again')).toBeInTheDocument();
            expect(screen.queryByLabelText('Replace')).not.toBeInTheDocument();
        } finally {
            onlineManager.setOnline(true);
        }
    });

    it('shares settings between sync option panels', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({syncWithReplaceAllowed: true} as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        render(
            <QueryClientProvider client={client}>
                <ApplicationSyncOptions options={[]} onChanged={jest.fn()} id='first' />
                <ApplicationSyncOptions options={[]} onChanged={jest.fn()} id='second' />
            </QueryClientProvider>
        );

        expect(await screen.findAllByLabelText('Replace')).toHaveLength(2);
        expect(services.authService.settings).toHaveBeenCalledTimes(1);
    });

    it('offers retry after settings fail to load', async () => {
        jest.mocked(services.authService.settings)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({syncWithReplaceAllowed: true} as any);
        const show = jest.fn();
        renderOptions({}, newQueryClient({show} as any));

        fireEvent.click(await screen.findByText('try again'));
        expect(await screen.findByLabelText('Replace')).toBeInTheDocument();
        expect(show).toHaveBeenCalledTimes(1);
    });
});
