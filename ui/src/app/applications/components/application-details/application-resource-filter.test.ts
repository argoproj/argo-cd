import {getEffectiveResourceFilter} from './application-resource-filter';

test('ApplicationSet ignores kind filters when applying resource filters', () => {
    expect(getEffectiveResourceFilter(false, ['kind:Deployment', 'sync:OutOfSync'])).toEqual(['sync:OutOfSync']);
});

test('Application still applies kind filters', () => {
    expect(getEffectiveResourceFilter(true, ['kind:Deployment', 'sync:OutOfSync'])).toEqual(['kind:Deployment', 'sync:OutOfSync']);
});
