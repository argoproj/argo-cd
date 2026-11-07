import * as models from '../../../shared/models';
import {countHiddenSelected, deselectHidden, getKindCounts, isVisible, selectionForLink} from './application-sync-panel-utils';

const res = (kind: string, name: string, status: models.SyncStatusCode = 'OutOfSync'): models.ResourceStatus =>
    ({group: kind === 'Deployment' ? 'apps' : '', version: 'v1', kind, namespace: 'ns', name, status}) as models.ResourceStatus;

// index:      0                         1                        2                         3                      4
const resources = [res('ConfigMap', 'cm', 'Synced'), res('Deployment', 'd1'), res('Deployment', 'd2', 'Synced'), res('Service', 's1'), res('Service', 's2', 'Synced')];

describe('getKindCounts', () => {
    test('returns sorted unique kinds with counts', () => {
        expect(getKindCounts(resources)).toEqual([
            {kind: 'ConfigMap', count: 1},
            {kind: 'Deployment', count: 2},
            {kind: 'Service', count: 2}
        ]);
    });

    test('returns empty list for no resources', () => {
        expect(getKindCounts([])).toEqual([]);
    });
});

describe('isVisible', () => {
    test('every resource is visible when no kind is selected', () => {
        expect(resources.every(r => isVisible(r, []))).toBe(true);
    });

    test('only resources of the selected kinds are visible', () => {
        expect(resources.map(r => isVisible(r, ['Deployment', 'ConfigMap']))).toEqual([true, true, true, false, false]);
    });
});

describe('selectionForLink', () => {
    const none = [false, false, false, false, false];
    const all = [true, true, true, true, true];

    test.each([
        ['all', none, [], all],
        ['outOfSync', all, [], [false, true, false, true, false]],
        ['none', all, [], none]
    ] as const)('with no kind filter %s keeps the legacy behaviour', (link, current, kinds, expected) => {
        expect(selectionForLink(resources, [...current], [...kinds], link)).toEqual(expected);
    });

    test('all selects only visible rows and keeps hidden rows as they were', () => {
        expect(selectionForLink(resources, [true, false, false, false, true], ['Deployment'], 'all')).toEqual([true, true, true, false, true]);
    });

    test('outOfSync replaces the selection of visible rows only', () => {
        expect(selectionForLink(resources, all, ['Deployment', 'Service'], 'outOfSync')).toEqual([true, true, false, true, false]);
    });

    test('none clears visible rows only', () => {
        expect(selectionForLink(resources, all, ['Service'], 'none')).toEqual([true, true, true, false, false]);
    });

    test('does not mutate the input selection', () => {
        const current = [true, true, true, true, true];
        selectionForLink(resources, current, ['Service'], 'none');
        expect(current).toEqual([true, true, true, true, true]);
    });
});

describe('hidden selected rows', () => {
    const selection = [true, true, false, true, true];

    test('countHiddenSelected counts checked rows outside the filter', () => {
        expect(countHiddenSelected(resources, selection, ['Deployment'])).toBe(3);
    });

    test('countHiddenSelected is 0 without a filter', () => {
        expect(countHiddenSelected(resources, selection, [])).toBe(0);
    });

    test('deselectHidden unchecks hidden rows and keeps visible ones', () => {
        expect(deselectHidden(resources, selection, ['Deployment'])).toEqual([false, true, false, false, false]);
    });
});
