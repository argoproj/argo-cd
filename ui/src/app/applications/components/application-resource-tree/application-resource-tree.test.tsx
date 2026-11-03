import * as React from 'react';
import {render} from '@testing-library/react';
import {ApplicationResourceTree, compareNodes, describeNode, ResourceTreeNode} from './application-resource-tree';

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

describe('ApplicationResourceTree pod grouping help tip', () => {
    const app = {
        kind: 'Application',
        metadata: {name: 'guestbook', namespace: 'argocd', resourceVersion: '1'},
        spec: {destination: {}, source: {repoURL: 'https://example.com/repo.git'}},
        status: {resources: [], sync: {status: 'Synced'}, health: {status: 'Healthy'}, summary: {}}
    } as any;

    const pods = (count: number) =>
        Array.from({length: count}, (_, i) => ({kind: 'Pod', group: '', version: 'v1', name: `pod-${i}`, namespace: 'default', uid: `uid-${i}`, resourceVersion: '1'}));

    const renderTree = (podCount: number, callbacks: {updateUsrHelpTipMsgs: jest.Mock; setShowCompactNodes: jest.Mock}) => (
        <ApplicationResourceTree
            app={app}
            tree={{nodes: pods(podCount)} as any}
            useNetworkingHierarchy={false}
            nodeFilter={() => true}
            onClearFilter={() => undefined}
            showOrphanedResources={false}
            showCompactNodes={false}
            userMsgs={[]}
            zoom={1}
            podGroupCount={15}
            nameDirection={false}
            nameWrap={false}
            setNodeExpansion={() => undefined}
            getNodeExpansion={() => false}
            {...callbacks}
        />
    );

    const newCallbacks = () => ({updateUsrHelpTipMsgs: jest.fn(), setShowCompactNodes: jest.fn()});

    it('does not show the help tip while the pod count is at or below the threshold', () => {
        const callbacks = newCallbacks();
        render(renderTree(15, callbacks));
        expect(callbacks.updateUsrHelpTipMsgs).not.toHaveBeenCalled();
        expect(callbacks.setShowCompactNodes).not.toHaveBeenCalled();
    });

    it('shows the help tip and switches to compact nodes once the pod count exceeds the threshold', () => {
        const callbacks = newCallbacks();
        render(renderTree(16, callbacks));
        expect(callbacks.updateUsrHelpTipMsgs).toHaveBeenCalledTimes(1);
        expect(callbacks.updateUsrHelpTipMsgs).toHaveBeenCalledWith(expect.objectContaining({appName: 'guestbook', msgKey: 'groupNodes'}));
        expect(callbacks.setShowCompactNodes).toHaveBeenCalledWith(true);
    });

    it('only re-runs when the pod count changes, using the latest callbacks', () => {
        const first = newCallbacks();
        const {rerender} = render(renderTree(16, first));
        expect(first.updateUsrHelpTipMsgs).toHaveBeenCalledTimes(1);

        // New callback instances on every render (as the parent creates them) must not re-trigger the effect.
        const second = newCallbacks();
        rerender(renderTree(16, second));
        expect(first.updateUsrHelpTipMsgs).toHaveBeenCalledTimes(1);
        expect(second.updateUsrHelpTipMsgs).not.toHaveBeenCalled();

        // A pod count change re-runs it with the callbacks from the latest render.
        const third = newCallbacks();
        rerender(renderTree(17, third));
        expect(second.updateUsrHelpTipMsgs).not.toHaveBeenCalled();
        expect(third.updateUsrHelpTipMsgs).toHaveBeenCalledTimes(1);
        expect(third.setShowCompactNodes).toHaveBeenCalledWith(true);
    });
});
