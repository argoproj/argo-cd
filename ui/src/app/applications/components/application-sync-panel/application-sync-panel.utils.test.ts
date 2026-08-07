import * as models from '../../../shared/models';
import {getAvailableKinds, setSelectionForKind} from './application-sync-panel.utils';

const r = (kind: string, name = 'res'): models.ResourceStatus =>
    ({kind, name, namespace: 'ns', status: 'OutOfSync', version: 'v1'} as unknown as models.ResourceStatus);

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

    describe('setSelectionForKind', () => {
        const resources = [r('Service', 's1'), r('Deployment', 'd1'), r('Service', 's2'), r('ConfigMap', 'c1')];

        it('sets matching kind to true and preserves others', () => {
            const selections = [false, true, false, true];
            expect(setSelectionForKind(resources, selections, 'Service', true)).toEqual([true, true, true, true]);
        });

        it('sets matching kind to false and preserves others', () => {
            const selections = [true, true, true, true];
            expect(setSelectionForKind(resources, selections, 'Service', false)).toEqual([false, true, false, true]);
        });

        it('preserves selections for non-matching kinds across calls (no clobber)', () => {
            let selections = [false, false, false, false];
            selections = setSelectionForKind(resources, selections, 'Service', true);
            expect(selections).toEqual([true, false, true, false]);
            selections = setSelectionForKind(resources, selections, 'Deployment', true);
            expect(selections).toEqual([true, true, true, false]);
            selections = setSelectionForKind(resources, selections, 'Service', false);
            expect(selections).toEqual([false, true, false, false]);
        });

        it('returns identity when kind matches nothing', () => {
            const selections = [true, false, true, false];
            expect(setSelectionForKind(resources, selections, 'Pod', true)).toEqual(selections);
        });

        it('returns empty when resources empty', () => {
            expect(setSelectionForKind([], [], 'Service', true)).toEqual([]);
        });
    });
});
