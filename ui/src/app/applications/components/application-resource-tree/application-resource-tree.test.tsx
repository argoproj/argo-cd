import {compareNodes, describeNode, getParentKey, ResourceTreeNode} from './application-resource-tree';
import {NodeId} from '../utils';

test('describeNode.NoImages', () => {
    expect(
        describeNode({
            kind: 'my-kind',
            name: 'my-name',
            namespace: 'my-ns',
        } as ResourceTreeNode),
    ).toBe(`Kind: my-kind
Namespace: my-ns
Name: my-name`);
});

test('describeNode.Images', () => {
    expect(
        describeNode({
            kind: 'my-kind',
            name: 'my-name',
            namespace: 'my-ns',
            images: ['my-image:v1'],
        } as ResourceTreeNode),
    ).toBe(`Kind: my-kind
Namespace: my-ns
Name: my-name
Images:
- my-image:v1`);
});

test('compareNodes', () => {
    const nodes = [
        {
            resourceVersion: '1',
            name: 'a',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:1',
                },
            ],
        } as ResourceTreeNode,
        {
            orphaned: false,
            resourceVersion: '1',
            name: 'a',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:1',
                },
            ],
        } as ResourceTreeNode,
        {
            orphaned: false,
            resourceVersion: '1',
            name: 'b',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:1',
                },
            ],
        } as ResourceTreeNode,
        {
            orphaned: false,
            resourceVersion: '2',
            name: 'a',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:2',
                },
            ],
        } as ResourceTreeNode,
        {
            orphaned: false,
            resourceVersion: '2',
            name: 'b',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:2',
                },
            ],
        } as ResourceTreeNode,
        {
            orphaned: true,
            resourceVersion: '1',
            name: 'a',
            info: [
                {
                    name: 'Revision',
                    value: 'Rev:1',
                },
            ],
        } as ResourceTreeNode,
    ];
    expect(compareNodes(nodes[0], nodes[1])).toBe(0);
    expect(compareNodes(nodes[2], nodes[1])).toBe(1);
    expect(compareNodes(nodes[1], nodes[2])).toBe(-1);
    expect(compareNodes(nodes[3], nodes[2])).toBe(-1);
    expect(compareNodes(nodes[2], nodes[3])).toBe(1);
    expect(compareNodes(nodes[4], nodes[3])).toBe(1);
    expect(compareNodes(nodes[3], nodes[4])).toBe(-1);
    expect(compareNodes(nodes[5], nodes[4])).toBe(1);
    expect(compareNodes(nodes[4], nodes[5])).toBe(-1);
    expect(compareNodes(nodes[0], nodes[4])).toBe(-1);
    expect(compareNodes(nodes[4], nodes[0])).toBe(1);
});

describe('getParentKey', () => {
    test('returns UID-based key when parent has UID and node exists', () => {
        const nodeByKey = new Map<string, ResourceTreeNode>();
        const node: ResourceTreeNode = {
            uid: 'test-uid-123',
            name: 'test-role',
            namespace: 'default',
            kind: 'Role',
            group: 'rbac.authorization.k8s.io',
            version: 'v1'
        } as ResourceTreeNode;

        nodeByKey.set('rbac.authorization.k8s.io/Role/default/test-role', node);

        const parent: NodeId & {uid?: string} = {
            group: 'rbac.authorization.k8s.io',
            kind: 'Role',
            namespace: 'default',
            name: 'test-role',
            uid: 'test-uid-123'
        };

        const result = getParentKey(parent, nodeByKey);
        expect(result).toBe('rbac.authorization.k8s.io/Role/default/test-role');
    });

    test('handles cluster-scoped parent with empty namespace', () => {
        const nodeByKey = new Map<string, ResourceTreeNode>();
        const clusterRole: ResourceTreeNode = {
            uid: 'cluster-uid-456',
            name: 'test-cluster-role',
            namespace: '', // cluster-scoped has no namespace
            kind: 'ClusterRole',
            group: 'rbac.authorization.k8s.io',
            version: 'v1'
        } as ResourceTreeNode;

        // Cluster-scoped resource is indexed without namespace
        nodeByKey.set('rbac.authorization.k8s.io/ClusterRole//test-cluster-role', clusterRole);

        const parent: NodeId & {uid?: string} = {
            group: 'rbac.authorization.k8s.io',
            kind: 'ClusterRole',
            namespace: '', // Backend now properly sets empty namespace for cluster-scoped parents
            name: 'test-cluster-role'
        };

        const result = getParentKey(parent, nodeByKey);
        // Should create the key with empty namespace
        expect(result).toBe('rbac.authorization.k8s.io/ClusterRole//test-cluster-role');
    });

    test('falls back to regular key when parent not found', () => {
        const nodeByKey = new Map<string, ResourceTreeNode>();

        const parent: NodeId & {uid?: string} = {
            group: 'apps',
            kind: 'Deployment',
            namespace: 'default',
            name: 'my-deployment'
        };

        const result = getParentKey(parent, nodeByKey);
        // Should return the regular key when not found
        expect(result).toBe('apps/Deployment/default/my-deployment');
    });

});
