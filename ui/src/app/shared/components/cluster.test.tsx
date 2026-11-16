import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {newQueryClient} from '../query-client';
import {services} from '../services';
import {Cluster, ClusterCtx} from './cluster';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: (props: {children: React.ReactNode}) => props.children}));
jest.mock('../services', () => ({services: {clusters: {get: jest.fn(), list: jest.fn()}}}));

const inCluster = {name: 'in-cluster', server: 'https://kubernetes.default.svc'} as any;
const prod = {name: 'prod', server: 'https://prod.example.com'} as any;

const renderClusters = (children: React.ReactNode, client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(<QueryClientProvider client={client}>{children}</QueryClientProvider>);

describe('Cluster', () => {
    beforeEach(() => jest.clearAllMocks());

    describe('without a cluster list', () => {
        it('looks up the cluster once for every Cluster showing it', async () => {
            jest.mocked(services.clusters.get).mockResolvedValue(prod);
            renderClusters(
                <>
                    <Cluster server={prod.server} />
                    <Cluster server={prod.server} showUrl={true} />
                </>
            );

            expect(screen.getAllByText(prod.server)).toHaveLength(2);
            expect(await screen.findByText('prod')).toBeInTheDocument();
            expect(screen.getByText(`prod (${prod.server})`)).toBeInTheDocument();
            expect(services.clusters.get).toHaveBeenCalledTimes(1);
            expect(services.clusters.get).toHaveBeenCalledWith(prod.server, undefined);
        });

        it('shows the server without an error when the lookup fails', async () => {
            jest.mocked(services.clusters.get).mockRejectedValue(new Error('forbidden'));
            const show = jest.fn();
            renderClusters(<Cluster server={prod.server} showUrl={true} />, newQueryClient({show} as any));

            expect(await screen.findByText(`${prod.server} (${prod.server})`)).toBeInTheDocument();
            expect(show).not.toHaveBeenCalled();
        });
    });

    describe('with a cluster list', () => {
        const renderWithList = (children: React.ReactNode, client?: QueryClient) =>
            renderClusters(<ClusterCtx.Provider value={Promise.resolve([])}>{children}</ClusterCtx.Provider>, client);

        it('finds clusters by server or name with one list request', async () => {
            jest.mocked(services.clusters.list).mockResolvedValue([inCluster, prod]);
            renderWithList(
                <>
                    <Cluster server={inCluster.server} />
                    <Cluster server='' name='prod' />
                    <Cluster server='https://unknown.example.com' />
                </>
            );

            expect(await screen.findByText('in-cluster')).toBeInTheDocument();
            expect(screen.getByText('prod')).toBeInTheDocument();
            expect(screen.getByText('https://unknown.example.com')).toBeInTheDocument();
            expect(services.clusters.list).toHaveBeenCalledTimes(1);
            expect(services.clusters.get).not.toHaveBeenCalled();
        });

        it('does not match a cluster without a name when no name is given', async () => {
            jest.mocked(services.clusters.list).mockResolvedValue([{server: 'https://nameless.example.com'} as any, prod]);
            renderWithList(<Cluster server='https://unknown.example.com' showUrl={true} />);

            expect(await screen.findByText('https://unknown.example.com (https://unknown.example.com)')).toBeInTheDocument();
        });

        it('does not refetch a failed list for rows mounting later', async () => {
            jest.mocked(services.clusters.list).mockRejectedValue(new Error('boom'));
            const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
            const {rerender} = renderWithList(<Cluster server={prod.server} />, client);
            await screen.findByText('try again');

            rerender(
                <QueryClientProvider client={client}>
                    <ClusterCtx.Provider value={Promise.resolve([])}>
                        <Cluster server={prod.server} />
                        <Cluster server={inCluster.server} />
                    </ClusterCtx.Provider>
                </QueryClientProvider>
            );

            expect(await screen.findAllByText('try again')).toHaveLength(2);
            expect(services.clusters.list).toHaveBeenCalledTimes(1);
        });

        it('offers retry when the list fails', async () => {
            jest.mocked(services.clusters.list).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce([prod]);
            const show = jest.fn();
            renderWithList(<Cluster server={prod.server} />, newQueryClient({show} as any));

            fireEvent.click(await screen.findByText('try again'));
            expect(await screen.findByText('prod')).toBeInTheDocument();
            expect(show).toHaveBeenCalledTimes(1);
        });
    });
});
