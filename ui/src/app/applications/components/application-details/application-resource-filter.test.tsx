import * as models from '../../../shared/models';
import * as React from 'react';
import {render, waitFor} from '@testing-library/react';
import {AppDetailsPreferences} from '../../../shared/services';
import {Filters, getResourceCounts} from './application-resource-filter';

const resourceNodes = [
    {group: 'apps', kind: 'Deployment', name: 'healthy-deployment', status: 'Synced', health: {status: 'Healthy'}},
    {group: '', kind: 'Service', name: 'healthy-service', status: 'Synced', health: {status: 'Healthy'}},
    {group: 'apps', kind: 'Deployment', name: 'degraded-deployment', status: 'OutOfSync', health: {status: 'Degraded'}},
    {group: '', kind: 'Pod', name: 'healthy-pod-1', status: 'Synced', health: {status: 'Healthy'}},
    {group: '', kind: 'Pod', name: 'healthy-pod-2', status: 'Synced', health: {status: 'Healthy'}}
] as models.ResourceStatus[];

test('getResourceCounts counts the full resource set in one pass', () => {
    const counts = getResourceCounts(resourceNodes);

    expect(counts.sync.get('Synced')).toBe(4);
    expect(counts.sync.get('OutOfSync')).toBe(1);
    expect(counts.health.get('Healthy')).toBe(4);
    expect(counts.health.get('Degraded')).toBe(1);
    expect(counts.kind.get('Deployment')).toBe(2);
    expect(counts.kind.get('Service')).toBe(1);
    expect(counts.kind.get('Pod')).toBe(2);
});

test('Filters keeps full resource counts when a zero-result filter is selected', async () => {
    const {container} = render(
        <Filters
            pref={{resourceFilter: ['health:Suspended']} as AppDetailsPreferences}
            tree={{nodes: resourceNodes, orphanedNodes: []} as models.ApplicationTree}
            resourceNodes={resourceNodes}
            onSetFilter={() => undefined}
            onClearFilter={() => undefined}
        />
    );

    await waitFor(() => expect(container.textContent).not.toContain('Loading...'));

    const rows = Array.from(container.querySelectorAll('.filter__item'));
    const healthyRow = rows.find(row => row.querySelector('.filter__item__label')?.textContent === 'Healthy');
    const suspendedRow = rows.find(row => row.querySelector('.filter__item__label')?.textContent === 'Suspended');
    expect(healthyRow?.textContent).toContain('4');
    expect(suspendedRow?.textContent).toContain('0');
});