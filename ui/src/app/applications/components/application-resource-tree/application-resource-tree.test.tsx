import {compareNodes, describeNode, getEffectiveParentKey, ResourceTreeNode} from './application-resource-tree';

function node(kind: string, name: string, parent?: ResourceTreeNode): ResourceTreeNode {
    return {
        group: kind === 'Pod' ? '' : 'apps',
        version: 'v1',
        kind,
        name,
        namespace: 'demo',
        uid: `${kind}-${name}`,
        parentRefs: parent ? [{group: parent.group, kind: parent.kind, name: parent.name, namespace: parent.namespace, uid: parent.uid}] : []
    } as ResourceTreeNode;
}

// Mirrors the tree's compact-view rule: a Pod is grouped when it has a parent and is not managed.
const groupedUnlessManaged =
    (managed: Set<string> = new Set()) =>
    (n: ResourceTreeNode) =>
        n.kind === 'Pod' && (n.parentRefs || []).length > 0 && !managed.has(n.uid);

test('getEffectiveParentKey.podOwnedResourceAttachesToPodGroupParentWhenCompact', () => {
    const leaderSts = node('StatefulSet', 'demo-lws');
    const leaderPod = node('Pod', 'demo-lws-0', leaderSts);
    // nodeByKey is keyed by treeNodeKey(), which is the uid when set
    const byKey = new Map<string, ResourceTreeNode>([leaderSts, leaderPod].map(n => [n.uid, n]));

    // grouped view: a resource owned by the leader Pod attaches to the leader StatefulSet
    expect(getEffectiveParentKey(leaderPod, byKey, groupedUnlessManaged())).toBe(leaderSts.uid);
    // normal view: nothing is grouped, so keep the real parent (the leader Pod)
    expect(getEffectiveParentKey(leaderPod, byKey, () => false)).toBe(leaderPod.uid);
});

test('getEffectiveParentKey.managedPodWithParentIsKept', () => {
    // a managed Pod is rendered as a root even if it has an owner, so its children stay under it
    const owner = node('StatefulSet', 'owner');
    const managedPod = node('Pod', 'managed', owner);
    const byKey = new Map<string, ResourceTreeNode>([owner, managedPod].map(n => [n.uid, n]));
    expect(getEffectiveParentKey(managedPod, byKey, groupedUnlessManaged(new Set([managedPod.uid])))).toBe(managedPod.uid);
});

test('getEffectiveParentKey.rootPodIsKept', () => {
    // a Pod with no parent is rendered as its own node, so it stays the parent
    const rootPod = node('Pod', 'standalone');
    expect(getEffectiveParentKey(rootPod, new Map(), groupedUnlessManaged())).toBe(rootPod.uid);
});

test('getEffectiveParentKey.nonPodParentIsKept', () => {
    // a resource whose parent is not a Pod is unaffected, even in compact view
    const deploy = node('Deployment', 'web');
    const rs = node('ReplicaSet', 'web-abc', deploy);
    const byKey = new Map<string, ResourceTreeNode>([deploy, rs].map(n => [n.uid, n]));
    expect(getEffectiveParentKey(rs, byKey, groupedUnlessManaged())).toBe(rs.uid);
});

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
