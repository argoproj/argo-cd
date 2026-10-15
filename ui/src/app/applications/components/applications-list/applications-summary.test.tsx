import {act, render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';

import {ClusterCtx} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ApplicationsSummary} from './applications-summary';

jest.mock('react-svg-piechart', () => ({default: () => null}));
jest.mock('../utils', () => ({
    ComparisonStatusIcon: () => null,
    HealthStatusIcon: () => null,
    HydrateOperationPhaseIcon: () => null
}));

const application = (destination: models.ApplicationDestination) =>
    ({
        spec: {destination},
        status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}}
    }) as models.Application;

const renderSummary = (clusters: Promise<models.Cluster[]>, applications: models.Application[]) =>
    render(
        <ClusterCtx.Provider value={clusters}>
            <ApplicationsSummary applications={applications} />
        </ClusterCtx.Provider>
    );

const expectClusterCount = (count: number) => expect(screen.getByText('CLUSTERS').parentElement).toHaveTextContent(count.toString());

test('counts cluster aliases once and preserves destinations missing from the cluster list', async () => {
    const cluster = {name: 'prod', server: 'https://prod.example'} as models.Cluster;
    renderSummary(Promise.resolve([cluster]), [
        application({name: cluster.name}),
        application({server: cluster.server}),
        application({server: 'https://hidden-a.example'}),
        application({server: 'https://hidden-b.example'})
    ]);

    await waitFor(() => expectClusterCount(3));
});

test('falls back to application destinations when loading clusters fails', async () => {
    const clusters = Promise.reject(new Error('failed to load clusters'));
    renderSummary(clusters, [application({server: 'https://cluster-a.example'}), application({server: 'https://cluster-b.example'})]);

    await act(async () => {
        await clusters.catch(() => undefined);
    });

    expectClusterCount(2);
});
