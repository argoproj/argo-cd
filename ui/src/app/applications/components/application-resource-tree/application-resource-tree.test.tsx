import {compareNodes, describeNode, ResourceTreeNode} from './application-resource-tree';

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
