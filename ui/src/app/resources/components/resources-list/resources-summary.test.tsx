import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import * as React from 'react';

import {ClusterCtx} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ResourcesSummary} from './resources-summary';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: (props: {children: React.ReactNode}) => props.children}));
jest.mock('react-svg-piechart', () => ({default: () => null}));
jest.mock('../../../applications/components/utils', () => ({
    ComparisonStatusIcon: () => null,
    HealthStatusIcon: () => null
}));
jest.mock('../../../shared/services', () => ({services: {clusters: {list: jest.fn()}}}));

const prod = {name: 'prod', server: 'https://prod.example'} as models.Cluster;

// The same cluster, once by name and once by server.
const resources = [
    {appName: 'guestbook', appNamespace: 'argocd', kind: 'Deployment', namespace: 'default', status: 'Synced', clusterName: prod.name},
    {appName: 'guestbook', appNamespace: 'argocd', kind: 'Service', namespace: 'default', status: 'Synced', clusterServer: prod.server}
] as models.Resource[];

// Renders the summary as the Resources page does: inside a list page, reading the cluster list
// from react-query.
const renderSummary = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <ClusterCtx.Provider value={true}>
                <ResourcesSummary resources={resources} />
            </ClusterCtx.Provider>
        </QueryClientProvider>
    );

const clusterCount = () => screen.getByText('CLUSTERS').parentElement;

beforeEach(() => jest.clearAllMocks());

test('shows the summary once the cluster list is loaded', async () => {
    jest.mocked(services.clusters.list).mockResolvedValue([prod]);
    renderSummary();

    expect(screen.getByText('Loading...')).toBeInTheDocument();
    expect(await screen.findByText('CLUSTERS')).toBeInTheDocument();
    expect(clusterCount()).toHaveTextContent('1');
    expect(services.clusters.list).toHaveBeenCalledTimes(1);
});

test('recovers when the cluster list is loaded after a failure', async () => {
    jest.mocked(services.clusters.list).mockRejectedValueOnce(new Error('failed to load clusters')).mockResolvedValueOnce([prod]);
    renderSummary();

    fireEvent.click(await screen.findByText('try again'));

    expect(await screen.findByText('CLUSTERS')).toBeInTheDocument();
    expect(clusterCount()).toHaveTextContent('1');
    expect(services.clusters.list).toHaveBeenCalledTimes(2);
});
