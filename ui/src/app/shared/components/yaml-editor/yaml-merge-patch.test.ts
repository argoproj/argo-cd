import {buildYamlMergePatch} from './yaml-merge-patch';

describe('buildYamlMergePatch', () => {
    const snapshot = {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
            name: 'guestbook-ui',
            resourceVersion: '100',
            annotations: {tick: '1'}
        },
        spec: {replicas: 1},
        status: {readyReplicas: 1}
    };

    test('replica-only edit omits resourceVersion and status', () => {
        const yaml = [
            'apiVersion: apps/v1',
            'kind: Deployment',
            'metadata:',
            '  name: guestbook-ui',
            '  resourceVersion: "100"',
            '  annotations:',
            '    tick: "1"',
            'spec:',
            '  replicas: 3',
            'status:',
            '  readyReplicas: 1',
            ''
        ].join('\n');

        const patch = buildYamlMergePatch(snapshot, yaml) as any;

        expect(patch.spec).toEqual({replicas: 3});
        expect(patch.metadata).toBeUndefined();
        expect(patch.status).toBeUndefined();
        expect(JSON.stringify(patch)).not.toContain('resourceVersion');
    });

    test('uses the snapshot even if live object has moved on', () => {
        const live = {
            ...snapshot,
            metadata: {...snapshot.metadata, resourceVersion: '200', annotations: {tick: '9'}},
            status: {readyReplicas: 2}
        };
        const yaml = [
            'apiVersion: apps/v1',
            'kind: Deployment',
            'metadata:',
            '  name: guestbook-ui',
            '  resourceVersion: "100"',
            '  annotations:',
            '    tick: "1"',
            'spec:',
            '  replicas: 3',
            'status:',
            '  readyReplicas: 1',
            ''
        ].join('\n');

        const fromSnapshot = buildYamlMergePatch(snapshot, yaml) as any;
        const fromLive = buildYamlMergePatch(live, yaml) as any;

        expect(fromSnapshot.spec).toEqual({replicas: 3});
        expect(fromSnapshot.metadata).toBeUndefined();
        expect(fromSnapshot.status).toBeUndefined();
        expect(fromLive.metadata?.resourceVersion).toBe('100');
        expect(fromLive.status?.readyReplicas).toBe(1);
    });
});
