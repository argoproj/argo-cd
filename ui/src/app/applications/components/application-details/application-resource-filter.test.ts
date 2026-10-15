import {getEffectiveResourceFilter} from './application-resource-filter';

test('ApplicationSet ignores kind, sync and health filters when applying resource filters', () => {
    expect(getEffectiveResourceFilter(true, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded', 'name:foo'])).toEqual(['name:foo']);
});

test('ApplicationSet keeps non kind/sync/health filters', () => {
    expect(getEffectiveResourceFilter(true, ['namespace:default', 'name:foo'])).toEqual(['namespace:default', 'name:foo']);
});

test('Application still applies kind, sync and health filters', () => {
    expect(getEffectiveResourceFilter(false, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded'])).toEqual(['kind:Deployment', 'sync:OutOfSync', 'health:Degraded']);
});
