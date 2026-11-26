import {Pod} from '../../../shared/models';

import {getPodHealthCounts} from './pod-view';

describe('getPodHealthCounts', () => {
    it('counts healthy, degraded, and progressing pods', () => {
        const pods = [{health: 'Healthy'}, {health: 'Healthy'}, {health: 'Degraded'}, {health: 'Progressing'}, {health: 'Progressing'}] as Pod[];

        expect(getPodHealthCounts(pods)).toEqual({
            healthy: 2,
            degraded: 1,
            progressing: 2
        });
    });

    it('counts suspended pods as healthy', () => {
        const pods = [{health: 'Healthy'}, {health: 'Suspended'}, {health: 'Suspended'}] as Pod[];

        expect(getPodHealthCounts(pods)).toEqual({
            healthy: 3,
            degraded: 0,
            progressing: 0
        });
    });

    it('returns zero counts for an empty pod list', () => {
        expect(getPodHealthCounts([])).toEqual({
            healthy: 0,
            degraded: 0,
            progressing: 0
        });
    });

    it('ignores pod health states that do not use summary icons', () => {
        const pods = [{health: 'Unknown'}, {health: 'Missing'}] as Pod[];

        expect(getPodHealthCounts(pods)).toEqual({
            healthy: 0,
            degraded: 0,
            progressing: 0
        });
    });
});
