import * as models from '../../../shared/models';
import {getEffectiveResourceFilter, getKindResourceCount} from './application-resource-filter';

test('ApplicationSet ignores kind filters when applying resource filters', () => {
    expect(getEffectiveResourceFilter(false, ['kind:Deployment', 'sync:OutOfSync'])).toEqual(['sync:OutOfSync']);
});

test('Application still applies kind filters', () => {
    expect(getEffectiveResourceFilter(true, ['kind:Deployment', 'sync:OutOfSync'])).toEqual(['kind:Deployment', 'sync:OutOfSync']);
});

test('Kind filter count matches the number of resources of that kind', () => {
    const nodes: models.ResourceStatus[] = [
        {kind: 'Pod', group: '', version: 'v1', namespace: 'default', name: 'pod-1', status: 'Synced', health: {status: 'Healthy', message: ''}},
        {kind: 'Deployment', group: 'apps', version: 'v1', namespace: 'default', name: 'deploy-1', status: 'Synced', health: {status: 'Healthy', message: ''}}
    ];

    expect(getKindResourceCount(nodes, 'Pod')).toBe(1);
    expect(getKindResourceCount(nodes, 'Deployment')).toBe(1);
    expect(getKindResourceCount(nodes, 'Service')).toBe(0);
});
