import {nodeKey} from '../utils';
import * as models from '../../../shared/models';

/**
 * Tests for the filtering logic in ApplicationResourceList component.
 * 
 * The component filters resources based on whether they exist in the tree,
 * preventing crashes when resources are deleted but still referenced in state.
 */

const createMockResource = (name: string, kind: string = 'ReplicaSet', namespace: string = 'default'): models.ResourceStatus => ({
    name,
    kind,
    namespace,
    group: 'apps',
    version: 'v1',
    status: 'Synced' as models.SyncStatusCode,
    health: {status: 'Healthy'} as models.HealthStatus
});

const createMockNode = (name: string, kind: string = 'ReplicaSet', namespace: string = 'default'): models.ResourceNode => ({
    name,
    kind,
    namespace,
    group: 'apps',
    version: 'v1',
    uid: `uid-${name}`,
    resourceVersion: '1',
    parentRefs: [],
    info: [{name: 'Revision', value: 'Rev:1'}]
});

describe('ApplicationResourceList filtering logic', () => {
    describe('filterDeletedResources', () => {
        // This simulates the filtering logic used in the component:
        // const resourcesToSort = props.resources.filter(res => nodeByKey.has(nodeKey(res)));
        
        const filterResources = (resources: models.ResourceStatus[], nodes: models.ResourceNode[]): models.ResourceStatus[] => {
            const nodeByKey = new Map<string, models.ResourceNode>();
            nodes.forEach(node => nodeByKey.set(nodeKey(node), node));
            return resources.filter(res => nodeByKey.has(nodeKey(res)));
        };

        it('should filter out resources that do not exist in the tree', () => {
            const resources = [
                createMockResource('rs-1'),
                createMockResource('rs-2'),
                createMockResource('rs-3')
            ];

            // Tree only contains rs-1 and rs-3, rs-2 has been deleted
            const nodes = [
                createMockNode('rs-1'),
                createMockNode('rs-3')
            ];

            const filtered = filterResources(resources, nodes);

            expect(filtered).toHaveLength(2);
            expect(filtered.map(r => r.name)).toEqual(['rs-1', 'rs-3']);
            expect(filtered.find(r => r.name === 'rs-2')).toBeUndefined();
        });

        it('should return empty array when all resources are deleted from tree', () => {
            const resources = [
                createMockResource('rs-1'),
                createMockResource('rs-2')
            ];

            // Tree is empty - all resources deleted
            const nodes: models.ResourceNode[] = [];

            const filtered = filterResources(resources, nodes);

            expect(filtered).toHaveLength(0);
        });

        it('should return all resources when all exist in tree', () => {
            const resources = [
                createMockResource('rs-1'),
                createMockResource('rs-2'),
                createMockResource('rs-3')
            ];

            const nodes = [
                createMockNode('rs-1'),
                createMockNode('rs-2'),
                createMockNode('rs-3')
            ];

            const filtered = filterResources(resources, nodes);

            expect(filtered).toHaveLength(3);
            expect(filtered.map(r => r.name)).toEqual(['rs-1', 'rs-2', 'rs-3']);
        });

        it('should handle resources with different namespaces correctly', () => {
            const resources = [
                createMockResource('rs-1', 'ReplicaSet', 'namespace-a'),
                createMockResource('rs-1', 'ReplicaSet', 'namespace-b')
            ];

            // Only the one in namespace-a exists
            const nodes = [
                createMockNode('rs-1', 'ReplicaSet', 'namespace-a')
            ];

            const filtered = filterResources(resources, nodes);

            expect(filtered).toHaveLength(1);
            expect(filtered[0].namespace).toBe('namespace-a');
        });

        it('should handle resources with different kinds correctly', () => {
            const resources = [
                createMockResource('my-resource', 'ReplicaSet'),
                createMockResource('my-resource', 'Deployment')
            ];

            // Only the ReplicaSet exists
            const nodes = [
                createMockNode('my-resource', 'ReplicaSet')
            ];

            const filtered = filterResources(resources, nodes);

            expect(filtered).toHaveLength(1);
            expect(filtered[0].kind).toBe('ReplicaSet');
        });
    });

    describe('safe info access', () => {
        it('should handle node without info property', () => {
            const nodeWithoutInfo: models.ResourceNode = {
                name: 'rs-1',
                kind: 'ReplicaSet',
                namespace: 'default',
                group: 'apps',
                version: 'v1',
                uid: 'uid-rs-1',
                resourceVersion: '1',
                parentRefs: []
                // info is intentionally omitted
            };

            // Simulates the optional chaining: (node?.info || [])
            const info = nodeWithoutInfo.info || [];
            
            expect(info).toEqual([]);
            expect(() => info.filter(tag => !tag.name.includes('Node'))).not.toThrow();
        });

        it('should handle node with undefined info', () => {
            const nodeWithUndefinedInfo = {
                name: 'rs-1',
                kind: 'ReplicaSet',
                namespace: 'default',
                group: 'apps',
                version: 'v1',
                uid: 'uid-rs-1',
                resourceVersion: '1',
                parentRefs: [],
                info: undefined
            } as models.ResourceNode;

            // Simulates: ((nodeByKey.get(nodeKey(res)) as ResourceNode)?.info || [])
            const info = nodeWithUndefinedInfo?.info || [];
            
            expect(info).toEqual([]);
        });

        it('should safely access info when node is undefined', () => {
            const nodeByKey = new Map<string, models.ResourceNode>();
            const missingNode = nodeByKey.get('non-existent-key');

            // Simulates: ((nodeByKey.get(nodeKey(res)) as ResourceNode)?.info || [])
            const info = (missingNode as models.ResourceNode)?.info || [];
            
            expect(info).toEqual([]);
            expect(() => info.filter(tag => !tag.name.includes('Node'))).not.toThrow();
        });
    });
});
