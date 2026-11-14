import {Application, ApplicationTree, ResourceNode} from '../../../shared/models';
import {processTree} from './pod-view';

function node(kind: string, name: string, uid: string, parentUid?: string, extra: any = {}): ResourceNode {
    return {
        group: kind === 'Pod' ? '' : 'apps',
        version: 'v1',
        kind,
        name,
        namespace: 'default',
        uid,
        parentRefs: parentUid ? [{uid: parentUid, kind: 'ReplicaSet', name: 'web-1'} as any] : [],
        info: [],
        resourceVersion: '1',
        ...extra
    };
}

const tree = {
    nodes: [
        node('Deployment', 'web', 'deploy'),
        node('ReplicaSet', 'web-1', 'rs', 'deploy'),
        node('Pod', 'web-1-a', 'pod-a', 'rs', {health: {status: 'Healthy'}, info: [{name: 'Node', value: 'node-1'}]}),
        node('Pod', 'web-1-b', 'pod-b', 'rs', {health: {status: 'Degraded'}, info: [{name: 'Node', value: 'node-2'}]})
    ],
    hosts: [
        {name: 'node-1', systemInfo: {kernelVersion: '', operatingSystem: 'linux', architecture: 'amd64'}, resourcesInfo: []},
        {name: 'node-2', systemInfo: {kernelVersion: '', operatingSystem: 'linux', architecture: 'amd64'}, resourcesInfo: []}
    ]
} as unknown as ApplicationTree;

const app = {status: {resources: []}} as unknown as Application;
const noop = (): null => null;

describe('processTree', () => {
    test('shows all pods without a filter', () => {
        const groups = processTree('topLevelResource', [], tree, app, noop, noop);
        expect(groups.map(g => g.name)).toEqual(['web']);
        expect(groups[0].pods.map(p => p.name)).toEqual(['web-1-a', 'web-1-b']);
    });

    test('only includes pods matching the filter', () => {
        const groups = processTree('parentResource', [], tree, app, noop, noop, pod => pod.name === 'web-1-b');
        expect(groups.map(g => g.name)).toEqual(['web-1']);
        expect(groups[0].pods.map(p => p.name)).toEqual(['web-1-b']);
    });

    test('hides groups without matching pods', () => {
        const groups = processTree('node', tree.hosts, tree, app, noop, noop, pod => pod.name === 'web-1-b');
        expect(groups.map(g => g.name)).toEqual(['node-2']);
        expect(processTree('node', tree.hosts, tree, app, noop, noop, () => false)).toEqual([]);
    });
});
