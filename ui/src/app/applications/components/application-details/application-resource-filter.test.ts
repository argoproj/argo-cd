import {getEffectiveResourceFilter, resourceNodeMatchesFilter} from './application-resource-filter';

const noFilter = {name: [] as string[], kind: [] as string[], health: [] as string[], sync: [] as string[], namespace: [] as string[]};

test('ApplicationSet ignores kind, sync and health filters when applying resource filters', () => {
    expect(getEffectiveResourceFilter(true, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded', 'name:foo'])).toEqual(['name:foo']);
});

test('ApplicationSet keeps non kind/sync/health filters', () => {
    expect(getEffectiveResourceFilter(true, ['namespace:default', 'name:foo'])).toEqual(['namespace:default', 'name:foo']);
});

test('Application still applies kind, sync and health filters', () => {
    expect(getEffectiveResourceFilter(false, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded'])).toEqual(['kind:Deployment', 'sync:OutOfSync', 'health:Degraded']);
});

test('health filter excludes hooks and hook children when health is missing, null, or not selected', () => {
    const degraded = {...noFilter, health: ['Degraded']};
    const missing = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true};
    const nullHealth = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true, health: null};
    const healthy = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true, health: {status: 'Healthy'}};

    expect(resourceNodeMatchesFilter({...missing, root: missing}, degraded)).toBe(false);
    expect(resourceNodeMatchesFilter({...nullHealth, root: nullHealth}, degraded)).toBe(false);
    expect(resourceNodeMatchesFilter({...healthy, root: healthy}, degraded)).toBe(false);
    expect(resourceNodeMatchesFilter({name: 'child', kind: 'Pod', namespace: 'ns', root: {hook: true}}, degraded)).toBe(false);
    expect(resourceNodeMatchesFilter({name: 'child', kind: 'Pod', namespace: 'ns', health: null, root: {hook: true, health: null}}, degraded)).toBe(false);
    expect(resourceNodeMatchesFilter({name: 'child', kind: 'Pod', namespace: 'ns', health: {status: 'Healthy'}, root: {hook: true}}, degraded)).toBe(false);
});

test('health filter includes a degraded hook and a degraded child of a hook with no health', () => {
    const degraded = {...noFilter, health: ['Degraded']};
    const hook = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true, health: {status: 'Degraded'}};

    expect(resourceNodeMatchesFilter({...hook, root: hook}, degraded)).toBe(true);
    expect(resourceNodeMatchesFilter({name: 'child', kind: 'Pod', namespace: 'ns', health: {status: 'Degraded'}, root: {hook: true}}, degraded)).toBe(true);
    expect(resourceNodeMatchesFilter({name: 'deploy', kind: 'Deployment', namespace: 'ns', health: {status: 'Healthy'}, root: {health: {status: 'Healthy'}}}, degraded)).toBe(false);
});

test('health filter with multiple statuses includes a matching hook and excludes a hook with no health', () => {
    const selected = {...noFilter, health: ['Degraded', 'Healthy']};
    const healthyHook = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true, health: {status: 'Healthy'}};
    const unhealthiedHook = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true};

    expect(resourceNodeMatchesFilter({...healthyHook, root: healthyHook}, selected)).toBe(true);
    expect(resourceNodeMatchesFilter({...unhealthiedHook, root: unhealthiedHook}, selected)).toBe(false);
});

test('hooks stay visible without a health filter and still honor name, kind, and namespace filters', () => {
    const hook = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true};
    const node = {...hook, root: hook};

    expect(resourceNodeMatchesFilter(node, noFilter)).toBe(true);
    expect(resourceNodeMatchesFilter(node, {...noFilter, name: ['mig*']})).toBe(true);
    expect(resourceNodeMatchesFilter(node, {...noFilter, name: ['other']})).toBe(false);
    expect(resourceNodeMatchesFilter(node, {...noFilter, kind: ['Job']})).toBe(true);
    expect(resourceNodeMatchesFilter(node, {...noFilter, kind: ['Deployment']})).toBe(false);
    expect(resourceNodeMatchesFilter(node, {...noFilter, namespace: ['ns']})).toBe(true);
    expect(resourceNodeMatchesFilter(node, {...noFilter, namespace: ['other']})).toBe(false);
});

test('sync filter includes hooks with empty status and folds Unknown into OutOfSync', () => {
    const hook = {name: 'migrate', kind: 'Job', namespace: 'ns', hook: true, status: ''};
    const outOfSync = {name: 'deploy', kind: 'Deployment', namespace: 'ns', status: 'OutOfSync'};
    const unknown = {name: 'deploy', kind: 'Deployment', namespace: 'ns', status: 'Unknown'};

    expect(resourceNodeMatchesFilter({...hook, root: hook}, {...noFilter, sync: ['Synced']})).toBe(true);
    expect(resourceNodeMatchesFilter({...outOfSync, root: outOfSync}, {...noFilter, sync: ['OutOfSync']})).toBe(true);
    expect(resourceNodeMatchesFilter({...unknown, root: unknown}, {...noFilter, sync: ['OutOfSync']})).toBe(true);
});
