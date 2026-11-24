import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';

import {Cluster, ClusterCtx} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {computeAppsListStats} from './applications-list-stats';
import {ApplicationsSummary} from './applications-summary';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: (props: {children: React.ReactNode}) => props.children}));
jest.mock('react-svg-piechart', () => ({default: () => null}));
jest.mock('../utils', () => ({
    ComparisonStatusIcon: () => null,
    HealthStatusIcon: () => null,
    HydrateOperationPhaseIcon: () => null
}));
jest.mock('../../../shared/services', () => ({services: {clusters: {list: jest.fn()}}}));

const application = (destination: models.ApplicationDestination) =>
    ({
        spec: {destination},
        status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}}
    }) as models.Application;

// Renders the summary as the Applications page does: inside a list page, reading the cluster list
// from react-query. Extra children (e.g. a <Cluster> row) share the same list.
const renderSummary = (applications: models.Application[], children?: React.ReactNode) =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <ClusterCtx.Provider value={true}>
                <ApplicationsSummary stats={computeAppsListStats(applications)} />
                {children}
            </ClusterCtx.Provider>
        </QueryClientProvider>
    );

const expectClusterCount = (count: number) => expect(screen.getByText('CLUSTERS').parentElement).toHaveTextContent(count.toString());

const prod = {name: 'prod', server: 'https://prod.example'} as models.Cluster;

beforeEach(() => jest.clearAllMocks());

test('counts cluster aliases once and preserves destinations missing from the cluster list', async () => {
    jest.mocked(services.clusters.list).mockResolvedValue([prod]);
    renderSummary([
        application({name: prod.name}),
        application({server: prod.server}),
        application({server: 'https://hidden-a.example'}),
        application({server: 'https://hidden-b.example'})
    ]);

    await waitFor(() => expectClusterCount(3));
});

test('falls back to application destinations when loading clusters fails', async () => {
    jest.mocked(services.clusters.list).mockRejectedValue(new Error('failed to load clusters'));
    renderSummary([application({server: 'https://cluster-a.example'}), application({server: 'https://cluster-b.example'})], <Cluster server={prod.server} />);

    // The row shows the failure once the list request has failed.
    await screen.findByText('try again');
    expectClusterCount(2);
});

test('updates when the cluster list is loaded after a failure', async () => {
    jest.mocked(services.clusters.list).mockRejectedValueOnce(new Error('failed to load clusters')).mockResolvedValueOnce([prod]);
    // The name and the server of the same cluster count as two destinations until the list is known.
    renderSummary([application({name: prod.name}), application({server: prod.server})], <Cluster server={prod.server} />);
    fireEvent.click(await screen.findByText('try again'));

    await waitFor(() => expectClusterCount(1));
    expect(services.clusters.list).toHaveBeenCalledTimes(2);
});
