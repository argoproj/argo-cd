import {
    getEffectiveResourceFilter,
    getPodStatusReason,
    getRootResource,
    nameMatchesWildcardFilters,
    parseResourceFilter,
    podMatchesFilter,
    splitResourceFilter,
    uniqueRootResources
} from './application-resource-filter';
import {ResourceNode} from '../../../shared/models';

test('ApplicationSet ignores kind, sync and health filters when applying resource filters', () => {
    expect(getEffectiveResourceFilter(true, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded', 'name:foo'])).toEqual(['name:foo']);
});

test('ApplicationSet keeps non kind/sync/health filters', () => {
    expect(getEffectiveResourceFilter(true, ['namespace:default', 'name:foo'])).toEqual(['namespace:default', 'name:foo']);
});

test('Application still applies kind, sync and health filters', () => {
    expect(getEffectiveResourceFilter(false, ['kind:Deployment', 'sync:OutOfSync', 'health:Degraded'])).toEqual(['kind:Deployment', 'sync:OutOfSync', 'health:Degraded']);
});

function node(kind: string, name: string, uid: string, parentUid?: string, extra: any = {}): ResourceNode & {[key: string]: any} {
    return {
        group: '',
        version: 'v1',
        kind,
        name,
        namespace: 'default',
        uid,
        parentRefs: parentUid ? [{uid: parentUid} as any] : [],
        info: [],
        resourceVersion: '1',
        ...extra
    };
}

function pod(name: string, statusReason: string, health = 'Healthy', parentUid = 'rs'): ResourceNode & {[key: string]: any} {
    return node('Pod', name, `uid-${name}`, parentUid, {
        health: {status: health},
        info: [
            {name: 'Status Reason', value: statusReason},
            {name: 'Node', value: 'node-1'}
        ]
    });
}

describe('splitResourceFilter', () => {
    test('splits on the first colon only', () => {
        expect(splitResourceFilter('podStatus:Init:CrashLoopBackOff')).toEqual(['podStatus', 'Init:CrashLoopBackOff']);
        expect(splitResourceFilter('sync:OutOfSync')).toEqual(['sync', 'OutOfSync']);
    });

    test('ignores items without a prefix', () => {
        expect(splitResourceFilter('OutOfSync')).toBeNull();
        expect(splitResourceFilter(':OutOfSync')).toBeNull();
    });
});

describe('parseResourceFilter', () => {
    test('groups values by prefix and keeps colons in values', () => {
        expect(parseResourceFilter(['name:web-*', 'kind:Pod', 'health:Degraded', 'sync:Synced', 'namespace:default', 'podStatus:Init:ExitCode:1', 'unknown:x'])).toEqual({
            name: ['web-*'],
            kind: ['Pod'],
            health: ['Degraded'],
            sync: ['Synced'],
            namespace: ['default'],
            podStatus: ['Init:ExitCode:1']
        });
    });

    test('handles empty input', () => {
        expect(parseResourceFilter(undefined)).toEqual({name: [], kind: [], health: [], sync: [], namespace: [], podStatus: []});
    });
});

describe('getEffectiveResourceFilter per view', () => {
    const filters = ['kind:Deployment', 'podStatus:Completed', 'health:Degraded'];

    test('pods view ignores kind filters and keeps pod status filters', () => {
        expect(getEffectiveResourceFilter(false, filters, 'pods')).toEqual(['podStatus:Completed', 'health:Degraded']);
    });

    test('other views ignore pod status filters', () => {
        expect(getEffectiveResourceFilter(false, filters, 'tree')).toEqual(['kind:Deployment', 'health:Degraded']);
        expect(getEffectiveResourceFilter(false, filters)).toEqual(['kind:Deployment', 'health:Degraded']);
    });

    test('ApplicationSet ignores pod status filters too', () => {
        expect(getEffectiveResourceFilter(true, [...filters, 'name:foo'])).toEqual(['name:foo']);
    });
});

describe('nameMatchesWildcardFilters', () => {
    test('supports * wildcards and is case insensitive', () => {
        expect(nameMatchesWildcardFilters('web-7d9f-abc', ['web-*'])).toBe(true);
        expect(nameMatchesWildcardFilters('WEB-1', ['web-*'])).toBe(true);
        expect(nameMatchesWildcardFilters('api-1', ['web-*', 'db'])).toBe(false);
        expect(nameMatchesWildcardFilters('db', ['web-*', 'db'])).toBe(true);
    });

    test('escapes regex characters other than *', () => {
        expect(nameMatchesWildcardFilters('a.b', ['a.b'])).toBe(true);
        expect(nameMatchesWildcardFilters('axb', ['a.b'])).toBe(false);
    });
});

describe('getPodStatusReason', () => {
    test('returns the Status Reason info item', () => {
        expect(getPodStatusReason(pod('p', 'CreateContainerConfigError'))).toBe('CreateContainerConfigError');
        expect(getPodStatusReason({info: []})).toBe('');
        expect(getPodStatusReason({})).toBe('');
    });
});

describe('getRootResource', () => {
    test('walks up to the top level resource', () => {
        const deploy = node('Deployment', 'web', 'deploy');
        const rs = node('ReplicaSet', 'web-1', 'rs', 'deploy');
        const p = pod('web-1-a', 'Running');
        const byUid = new Map([deploy, rs, p].map(n => [n.uid, n]));
        expect(getRootResource(p, byUid)).toBe(deploy);
        expect(getRootResource(deploy, byUid)).toBe(deploy);
    });

    test('stops at a missing parent and on cycles', () => {
        const orphan = pod('orphan', 'Running', 'Healthy', 'missing');
        expect(getRootResource(orphan, new Map([[orphan.uid, orphan]]))).toBe(orphan);

        const a = node('A', 'a', 'a', 'b');
        const b = node('B', 'b', 'b', 'a');
        const root = getRootResource(a, new Map([a, b].map(n => [n.uid, n])));
        expect([a, b]).toContain(root);
    });
});

describe('uniqueRootResources', () => {
    test('returns each top-level resource once, so sync counts match the other views', () => {
        const rollout = node('Rollout', 'demo', 'rollout', undefined, {status: 'OutOfSync'});
        const rs = node('ReplicaSet', 'demo-1', 'rs', 'rollout');
        const job = node('Job', 'migrate', 'job', undefined, {status: 'Synced'});
        const pods = [pod('demo-1-a', 'Running'), pod('demo-1-b', 'Completed'), pod('migrate-a', 'Completed', 'Healthy', 'job')];
        const byUid = new Map([rollout, rs, job, ...pods].map(n => [n.uid, n]));
        const roots = uniqueRootResources(pods, p => getRootResource(p, byUid));
        expect(roots).toEqual([rollout, job]);
        expect(roots.filter(r => r.status === 'OutOfSync')).toHaveLength(1);
    });

    test('skips pods without a root', () => {
        expect(uniqueRootResources([pod('a', 'Running')], () => undefined)).toEqual([]);
    });
});

describe('podMatchesFilter', () => {
    const running = pod('web-1', 'Running');
    const completed = pod('job-1', 'Completed', 'Healthy');
    const failing = pod('web-2', 'CreateContainerConfigError', 'Degraded');
    const synced = {status: 'Synced' as const};
    const outOfSync = {status: 'OutOfSync' as const};

    test('matches everything without filters', () => {
        expect(podMatchesFilter(running, synced, parseResourceFilter([]))).toBe(true);
    });

    test('filters by pod status reason', () => {
        const filter = parseResourceFilter(['podStatus:CreateContainerConfigError', 'podStatus:Terminating']);
        expect(podMatchesFilter(failing, synced, filter)).toBe(true);
        expect(podMatchesFilter(completed, synced, filter)).toBe(false);
    });

    test('filters by pod health, not the root health', () => {
        const filter = parseResourceFilter(['health:Degraded']);
        expect(podMatchesFilter(failing, {...synced, health: {status: 'Healthy'}}, filter)).toBe(true);
        expect(podMatchesFilter(running, {...synced, health: {status: 'Degraded'}}, filter)).toBe(false);
    });

    test('does not match a pod without health status, consistent with the health filter counts', () => {
        const noHealth = {...running, health: undefined};
        expect(podMatchesFilter(noHealth, synced, parseResourceFilter(['health:Unknown']))).toBe(false);
    });

    test('filters by wildcard name and namespace', () => {
        expect(podMatchesFilter(running, synced, parseResourceFilter(['name:web-*']))).toBe(true);
        expect(podMatchesFilter(completed, synced, parseResourceFilter(['name:web-*']))).toBe(false);
        expect(podMatchesFilter(running, synced, parseResourceFilter(['namespace:other']))).toBe(false);
    });

    test('filters by sync status of the root resource', () => {
        const filter = parseResourceFilter(['sync:OutOfSync']);
        expect(podMatchesFilter(running, outOfSync, filter)).toBe(true);
        expect(podMatchesFilter(running, {status: 'Unknown'}, filter)).toBe(true);
        expect(podMatchesFilter(running, synced, filter)).toBe(false);
        expect(podMatchesFilter(running, undefined, filter)).toBe(false);
        expect(podMatchesFilter(running, {hook: true}, filter)).toBe(true);
    });
});
