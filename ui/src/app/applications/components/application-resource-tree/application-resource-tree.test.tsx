import * as models from '../../../shared/models';
import {compareNodes, describeNode, getNodeRenderLeft, getNodeRenderTop, getPodGroupHeight, NODE_HEIGHT, ResourceTreeNode} from './application-resource-tree';

const POD_NODE_HEIGHT = 136;

function pods(health: string, count: number): models.Pod[] {
    return Array.from({length: count}, (_, i) => ({name: `${health}-${i}`, health} as models.Pod));
}

describe('getPodGroupHeight', () => {
    test('reserves nothing for a pod group with no renderable pods', () => {
        expect(getPodGroupHeight([], true)).toBe(POD_NODE_HEIGHT);
        expect(getPodGroupHeight(undefined, false)).toBe(POD_NODE_HEIGHT);
        expect(getPodGroupHeight(pods('Unknown', 12), false)).toBe(POD_NODE_HEIGHT);
    });

    test('reserves one summary row per non-empty health bucket in the status view', () => {
        expect(getPodGroupHeight(pods('Healthy', 200), true)).toBe(POD_NODE_HEIGHT + 20);
        expect(getPodGroupHeight([...pods('Healthy', 3), ...pods('Degraded', 4)], true)).toBe(POD_NODE_HEIGHT + 40);
        expect(getPodGroupHeight([...pods('Healthy', 1), ...pods('Degraded', 1), ...pods('Progressing', 1)], true)).toBe(POD_NODE_HEIGHT + 60);
    });

    test('counts pod rows per bucket, since each bucket is drawn in its own container', () => {
        expect(getPodGroupHeight(pods('Healthy', 8), false)).toBe(POD_NODE_HEIGHT + 31);
        expect(getPodGroupHeight(pods('Healthy', 9), false)).toBe(POD_NODE_HEIGHT + 62);
        // Two half-full buckets occupy two rows, not ceil(8 / 8) = 1.
        expect(getPodGroupHeight([...pods('Healthy', 4), ...pods('Degraded', 4)], false)).toBe(POD_NODE_HEIGHT + 62);
    });
});

describe('node render position helpers', () => {
    test('convert the Dagre box center to the box top-left corner', () => {
        expect(getNodeRenderLeft({x: 200, width: 282})).toBe(59);
        expect(getNodeRenderTop({y: 200, height: NODE_HEIGHT})).toBe(200 - NODE_HEIGHT / 2);
        expect(getNodeRenderTop({y: 200, height: 156})).toBe(122);
    });

    test('preserve the Dagre gap between a regular node and the pod group below it', () => {
        const nodesep = 25;
        const regularNode = {y: 100, height: NODE_HEIGHT};
        const regularNodeBottom = getNodeRenderTop(regularNode) + regularNode.height;

        const podGroupHeight = 176;
        const podGroup = {y: regularNode.y + NODE_HEIGHT / 2 + nodesep + podGroupHeight / 2, height: podGroupHeight};

        expect(getNodeRenderTop(podGroup) - regularNodeBottom).toBe(nodesep);
    });
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
