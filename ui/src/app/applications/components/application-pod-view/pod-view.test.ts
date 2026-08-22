import {Application, ApplicationTree, Node, ResourceName} from '../../../shared/models';
import {PodGroup, processTree} from './pod-view';

function bareApp(): Application {
    return {
        metadata: {name: 'app', namespace: 'argocd'},
        spec: {destination: {}, project: 'default', source: {repoURL: ''}},
        status: {resources: []}
    } as Application;
}

function nodeInfo(kernel: string, os: string, arch: string) {
    return {
        name: 'worker-1',
        systemInfo: {
            architecture: arch,
            operatingSystem: os,
            kernelVersion: kernel
        },
        resourcesInfo: [],
        labels: {role: 'worker'}
    } as Node;
}

describe('processTree unschedulable group', () => {
    test('omits Kernel Version / OS-Arch / N/A metadata for Unschedulable node group', () => {
        const tree: ApplicationTree = {
            nodes: [
                {
                    kind: 'Pod',
                    name: 'pending-pod',
                    namespace: 'default',
                    uid: 'pod-1',
                    version: 'v1',
                    group: '',
                    info: [
                        // No Node info entry => pod is treated as unschedulable
                        {name: 'Status Reason', value: 'Unschedulable'}
                    ],
                    parentRefs: [],
                    health: {status: 'Progressing'}
                } as any
            ]
        };

        const groups = processTree('node', [nodeInfo('5.15.0', 'linux', 'amd64')], tree, bareApp(), () => null, () => null);
        const unsched = groups.find(g => g.name === 'Unschedulable');
        expect(unsched).toBeDefined();
        expect(unsched!.type).toBe('node');
        expect(unsched!.kind).toBe('node');
        expect(unsched!.pods).toHaveLength(1);
        expect(unsched!.info).toEqual([]);
        // Guard against regressing to placeholder N/A rows
        const infoText = JSON.stringify(unsched!.info || []);
        expect(infoText).not.toMatch(/N\/A/i);
        expect(infoText).not.toMatch(/Kernel Version/i);
        expect(infoText).not.toMatch(/OS\/Arch/i);
        expect(unsched!.hostResourcesInfo).toEqual([]);
        expect(unsched!.hostLabels).toEqual({});
    });

    test('still shows Kernel Version and OS/Arch for real node groups', () => {
        const tree: ApplicationTree = {
            nodes: [
                {
                    kind: 'Pod',
                    name: 'running-pod',
                    namespace: 'default',
                    uid: 'pod-2',
                    version: 'v1',
                    group: '',
                    info: [{name: 'Node', value: 'worker-1'}],
                    parentRefs: [],
                    health: {status: 'Healthy'}
                } as any
            ]
        };

        const groups = processTree('node', [nodeInfo('5.15.0-generic', 'linux', 'amd64')], tree, bareApp(), () => null, () => null);
        const worker = groups.find(g => g.name === 'worker-1');
        expect(worker).toBeDefined();
        expect(worker!.info).toEqual(
            expect.arrayContaining([
                {name: 'Kernel Version', value: '5.15.0-generic'},
                {name: 'OS/Arch', value: 'linux/amd64'}
            ])
        );
        expect(worker!.pods).toHaveLength(1);
    });

    test('does not put scheduled pods into Unschedulable', () => {
        const tree: ApplicationTree = {
            nodes: [
                {
                    kind: 'Pod',
                    name: 'running-pod',
                    namespace: 'default',
                    uid: 'pod-3',
                    version: 'v1',
                    group: '',
                    info: [{name: 'Node', value: 'worker-1'}],
                    parentRefs: [],
                    health: {status: 'Healthy'}
                } as any
            ]
        };

        const groups = processTree('node', [nodeInfo('5.15.0', 'linux', 'amd64')], tree, bareApp(), () => null, () => null);
        expect(groups.find(g => g.name === 'Unschedulable')).toBeUndefined();
    });
});
