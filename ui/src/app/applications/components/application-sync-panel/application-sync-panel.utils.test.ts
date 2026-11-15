import * as models from '../../../shared/models';
import {computeSelection, filterResources, getAvailableKinds} from './application-sync-panel.utils';

const r = (kind: string, name = 'res', status: models.SyncStatus = models.SyncStatuses.OutOfSync): models.ResourceStatus =>
    ({kind, name, namespace: 'ns', status, version: 'v1'} as unknown as models.ResourceStatus);

describe('application-sync-panel.utils', () => {
    describe('getAvailableKinds', () => {
        it('returns sorted unique kinds', () => {
            const resources = [r('Service'), r('Deployment'), r('Service'), r('ConfigMap')];
            expect(getAvailableKinds(resources)).toEqual(['ConfigMap', 'Deployment', 'Service']);
        });

        it('filters out empty kinds', () => {
            const resources = [r('Service'), r(''), r('Deployment')];
            expect(getAvailableKinds(resources)).toEqual(['Deployment', 'Service']);
        });

        it('returns empty array when resources is empty', () => {
            expect(getAvailableKinds([])).toEqual([]);
        });

        it('locale-aware sort', () => {
            const resources = [r('Zorp'), r('apple'), r('Banana')];
            const result = getAvailableKinds(resources);
            expect(result).toEqual([...result].sort((a, b) => a.localeCompare(b)));
        });
    });

    describe('computeSelection and filterResources', () => {
        const resources = [
            r('Deployment', 'd-out', models.SyncStatuses.OutOfSync),
            r('Deployment', 'd-sync', models.SyncStatuses.Synced),
            r('ConfigMap', 'cm-out', models.SyncStatuses.OutOfSync),
            r('ConfigMap', 'cm-sync', models.SyncStatuses.Synced),
            r('Service', 'svc-sync', models.SyncStatuses.Synced)
        ];
        const indexed = resources.map((item, i) => ({item, i}));

        it('selects and filters only out-of-sync resources of a specific kind, deselecting other kinds', () => {
            const selection = computeSelection(resources, 'Deployment', 'outOfSync');
            expect(selection).toEqual([true, false, false, false, false]);

            const visible = filterResources(indexed, 'Deployment', 'outOfSync');
            expect(visible.map(v => v.item.name)).toEqual(['d-out']);
        });

        it('selects and filters all resources of a specific kind when statusFilter is all', () => {
            const selection = computeSelection(resources, 'Deployment', 'all');
            expect(selection).toEqual([true, true, false, false, false]);

            const visible = filterResources(indexed, 'Deployment', 'all');
            expect(visible.map(v => v.item.name)).toEqual(['d-out', 'd-sync']);
        });

        it('deselects all when statusFilter is none', () => {
            const selection = computeSelection(resources, 'Deployment', 'none');
            expect(selection).toEqual([false, false, false, false, false]);
        });

        it('selects and filters all out-of-sync resources across all kinds when kindFilter is empty', () => {
            const selection = computeSelection(resources, '', 'outOfSync');
            expect(selection).toEqual([true, false, true, false, false]);

            const visible = filterResources(indexed, '', 'outOfSync');
            expect(visible.map(v => v.item.name)).toEqual(['d-out', 'cm-out']);
        });

        it('selects and filters all resources across all kinds when both filters are all', () => {
            const selection = computeSelection(resources, '', 'all');
            expect(selection).toEqual([true, true, true, true, true]);

            const visible = filterResources(indexed, '', 'all');
            expect(visible.length).toBe(5);
        });
    });
});
