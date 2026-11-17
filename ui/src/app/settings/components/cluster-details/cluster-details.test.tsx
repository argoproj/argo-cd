import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {services} from '../../../shared/services';
import {ClusterDetails} from './cluster-details';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {toolbar: {actionMenu: {items: {title: string; action: () => void}[]}}; children?: React.ReactNode}) => (
        <>
            {props.toolbar.actionMenu.items.map(item => (
                <button key={item.title} onClick={item.action}>
                    {item.title}
                </button>
            ))}
            {props.children}
        </>
    ),
    EditablePanel: (props: {items: {title: string; view: React.ReactNode}[]}) =>
        props.items.map(item => (
            <div key={item.title}>
                {item.title}: {item.view}
            </div>
        ))
}));

jest.mock('../../../shared/services', () => ({
    services: {
        clusters: {get: jest.fn(), invalidateCache: jest.fn()},
        applications: {list: jest.fn(() => Promise.resolve({items: [{spec: {destination: {name: 'in-cluster'}}}, {spec: {destination: {name: 'other'}}}]}))}
    }
}));

const server = 'https://kubernetes.default.svc';
const cluster = (version: string) => ({
    name: 'in-cluster',
    server,
    config: {},
    info: {serverVersion: version, connectionState: {status: 'Successful', attemptedAt: '2026-10-24T00:00:00Z'}, cacheInfo: {}}
});

const renderDetails = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <MemoryRouter initialEntries={[`/settings/clusters/${encodeURIComponent(server)}`]}>
                <Route path='/settings/clusters/:server' component={ClusterDetails} />
            </MemoryRouter>
        </QueryClientProvider>
    );

describe('ClusterDetails', () => {
    beforeEach(() => jest.clearAllMocks());
    afterEach(() => jest.useRealTimers());

    it('renders the cluster and its application count', async () => {
        jest.mocked(services.clusters.get).mockResolvedValue(cluster('v1.33.0') as any);
        renderDetails();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('v1.33.0')).toBeInTheDocument();
        expect(await screen.findByRole('link', {name: '1'})).toBeInTheDocument();
        expect(services.clusters.get).toHaveBeenCalledWith(server, '');
    });

    it('shows the cluster returned by invalidate cache', async () => {
        jest.mocked(services.clusters.get).mockResolvedValue(cluster('v1.33.0') as any);
        jest.mocked(services.clusters.invalidateCache).mockResolvedValue(cluster('v1.34.0') as any);
        renderDetails();

        fireEvent.click(await screen.findByText('Invalidate Cache'));

        expect(await screen.findByText('v1.34.0')).toBeInTheDocument();
    });

    it('stops polling once a request fails', async () => {
        jest.useFakeTimers();
        jest.mocked(services.clusters.get)
            .mockResolvedValueOnce(cluster('v1.33.0') as any)
            .mockRejectedValue(new Error('boom'));
        renderDetails();

        expect(await screen.findByText('v1.33.0')).toBeInTheDocument();
        await act(() => jest.advanceTimersByTimeAsync(1000));
        expect(await screen.findByText('try again')).toBeInTheDocument();
        await act(() => jest.advanceTimersByTimeAsync(5000));
        expect(services.clusters.get).toHaveBeenCalledTimes(2);
    });
});
