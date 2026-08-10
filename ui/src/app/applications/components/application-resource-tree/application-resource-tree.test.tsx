import {compareNodes, describeNode, mostRelevantFirst, ResourceTreeNode, rootStrategy, subtreeMatches, subtreeRelevance} from './application-resource-tree';

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

// A node identified by uid, which is what treeNodeKey uses, so a children map can be keyed by uid.
const node = (uid: string, extra: Partial<ResourceTreeNode> = {}) =>
    ({
        uid,
        name: uid,
        kind: 'ConfigMap',
        namespace: 'ns',
        version: 'v1',
        resourceVersion: '1',
        ...extra,
    }) as ResourceTreeNode;

const childrenOf = (pairs: {[parentUid: string]: ResourceTreeNode[]}) => new Map(Object.entries(pairs));

describe('mostRelevantFirst', () => {
    const rank = new Map<string, number>();
    const rankOf = (n: ResourceTreeNode) => rank.get(n.uid) ?? 6;

    beforeEach(() => rank.clear());

    test('keeps the most relevant up to the limit', () => {
        rank.set('degraded', 0).set('progressing', 1);
        const items = [node('healthy-b'), node('degraded'), node('healthy-a'), node('progressing')];
        expect(mostRelevantFirst(items, 2, rankOf).map(n => n.uid)).toEqual(['degraded', 'progressing']);
    });

    test('breaks ties within a tier by the tree ordering', () => {
        const items = [node('c'), node('a'), node('b')];
        expect(mostRelevantFirst(items, 2, rankOf).map(n => n.uid)).toEqual(['a', 'b']);
    });

    test('orders the whole list when it fits inside the limit', () => {
        rank.set('urgent', 0);
        const items = [node('b'), node('urgent'), node('a')];
        expect(mostRelevantFirst(items, 10, rankOf).map(n => n.uid)).toEqual(['urgent', 'a', 'b']);
    });

    test('a later item displaces an earlier one it outranks', () => {
        rank.set('late', 0);
        const items = [node('a'), node('b'), node('late')];
        expect(mostRelevantFirst(items, 1, rankOf).map(n => n.uid)).toEqual(['late']);
    });

    test('leaves the caller its array, which is cached elsewhere', () => {
        const items = [node('b'), node('a')];
        mostRelevantFirst(items, 10, rankOf);
        expect(items.map(n => n.uid)).toEqual(['b', 'a']);
    });
});

describe('subtreeMatches', () => {
    const matches = (uid: string) => (candidate: ResourceTreeNode) => candidate.uid === uid;

    test('matches the node itself', () => {
        expect(subtreeMatches(node('a'), matches('a'), childrenOf({}))).toBe(true);
    });

    test('matches a descendant, so the path down to it is kept', () => {
        const grandchild = node('grandchild');
        const child = node('child');
        const root = node('root');
        const children = childrenOf({root: [child], child: [grandchild]});
        expect(subtreeMatches(root, matches('grandchild'), children)).toBe(true);
    });

    test('reports nothing when neither the node nor its descendants match', () => {
        const children = childrenOf({root: [node('child')]});
        expect(subtreeMatches(node('root'), matches('elsewhere'), children)).toBe(false);
    });

    test('terminates on a cycle', () => {
        const a = node('a');
        const b = node('b');
        const children = childrenOf({a: [b], b: [a]});
        expect(subtreeMatches(a, matches('nothing'), children)).toBe(false);
    });
});

describe('subtreeRelevance', () => {
    test('a parent is as relevant as its least healthy descendant', () => {
        const degraded = node('degraded', {health: {status: 'Degraded'}} as Partial<ResourceTreeNode>);
        const children = childrenOf({root: [node('child')], child: [degraded]});
        expect(subtreeRelevance(node('root'), children)).toBe(0);
    });

    test('an untroubled subtree stays unremarkable', () => {
        const children = childrenOf({root: [node('child')]});
        expect(subtreeRelevance(node('root'), children)).toBe(6);
    });

    test('terminates on a cycle', () => {
        const a = node('a');
        const b = node('b');
        const children = childrenOf({a: [b], b: [a]});
        expect(subtreeRelevance(a, children)).toBe(6);
    });
});

describe('rootStrategy', () => {
    const CAP = 200;

    test('ranks roots when the budget can only bite through depth', () => {
        // 150 deployments, each with a replica set and three pods: far fewer roots than the cap, far more
        // nodes. Ordering by name here drops the later chains unranked and hides filter matches inside them.
        expect(rootStrategy(150, 750, CAP, false)).toEqual({clusterKinds: false, rankRoots: true});
    });

    test('leaves a small application in its familiar order', () => {
        expect(rootStrategy(10, 40, CAP, false)).toEqual({clusterKinds: false, rankRoots: false});
    });

    test('ranks whenever a filter is active, however small the application', () => {
        expect(rootStrategy(10, 40, CAP, true)).toEqual({clusterKinds: false, rankRoots: true});
    });

    test('clusters kinds only when the top level itself is crowded', () => {
        expect(rootStrategy(1500, 4000, CAP, false)).toEqual({clusterKinds: true, rankRoots: true});
    });

    test('drilling into a kind shows it flat, and ranked', () => {
        expect(rootStrategy(1500, 4000, CAP, false, 'ConfigMap')).toEqual({clusterKinds: false, rankRoots: true});
    });
});

describe('subtreeMatches with compacted pods', () => {
    const withPods = (uid: string, podNames: string[]) =>
        ({
            ...node(uid),
            kind: 'ReplicaSet',
            podGroup: {pods: podNames.map(name => ({name}))},
        }) as unknown as ResourceTreeNode;

    test('finds a pod that compact mode moved onto its parent', () => {
        const parent = withPods('rs', ['pod-a', 'pod-b']);
        const matchPod = (candidate: ResourceTreeNode) => candidate.kind === 'Pod' && candidate.name === 'pod-b';
        expect(subtreeMatches(parent, matchPod, childrenOf({}))).toBe(true);
    });

    test('keeps the path down to a grouped pod deeper in the tree', () => {
        const parent = withPods('rs', ['pod-a']);
        const matchPod = (candidate: ResourceTreeNode) => candidate.kind === 'Pod' && candidate.name === 'pod-a';
        expect(subtreeMatches(node('deploy'), matchPod, childrenOf({deploy: [parent]}))).toBe(true);
    });

    test('reports nothing when no grouped pod matches', () => {
        const parent = withPods('rs', ['pod-a']);
        const matchPod = (candidate: ResourceTreeNode) => candidate.name === 'pod-z';
        expect(subtreeMatches(node('deploy'), matchPod, childrenOf({deploy: [parent]}))).toBe(false);
    });
});
