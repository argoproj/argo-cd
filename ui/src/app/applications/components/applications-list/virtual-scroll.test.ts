import {renderHook} from '@testing-library/react';
import {
    appsLayoutKey,
    bidirectionalOverscanIndicesGetter,
    computeColumnWidth,
    computeColumnWidthForIndex,
    computeColumnsPerRow,
    computeOverscanRowCount,
    getTableRowHeight,
    hasActiveHydrator,
    OVERSCAN_VIEWPORT_SCREENS,
    shouldUseVirtualScroll,
    TABLE_OVERSCAN_ROW_COUNT,
    TABLE_ROW_HEIGHT,
    TABLE_ROW_HEIGHT_WITH_HYDRATOR,
    TILE_GAP,
    TILE_MIN_WIDTH,
    TILE_OVERSCAN_ROW_COUNT,
    TILE_ROW_STRIDE,
    useWindowScrollerPosition,
    VIRTUAL_THRESHOLD
} from './virtual-scroll';
import {Application} from '../../../shared/models';

describe('virtual-scroll', () => {
    const app = (name: string, hydrator = false): Application =>
        ({
            kind: 'Application',
            metadata: {name, namespace: 'argocd', uid: `uid-${name}`},
            spec: {project: 'default', destination: {}, source: {}},
            status: {
                health: {status: 'Healthy'},
                sync: {status: 'Synced'},
                ...(hydrator ? {sourceHydrator: {currentOperation: {phase: 'Running'}}} : {})
            }
        }) as Application;

    describe('overscan defaults', () => {
        it('keeps a floor for tile/table overscan', () => {
            expect(TILE_OVERSCAN_ROW_COUNT).toBeGreaterThanOrEqual(6);
            expect(TABLE_OVERSCAN_ROW_COUNT).toBeGreaterThanOrEqual(10);
            expect(OVERSCAN_VIEWPORT_SCREENS).toBeGreaterThan(0);
            expect(OVERSCAN_VIEWPORT_SCREENS).toBeLessThanOrEqual(2);
        });
    });

    describe('computeOverscanRowCount', () => {
        it('returns the minimum when viewport or row height is invalid', () => {
            expect(computeOverscanRowCount(0, TILE_ROW_STRIDE, TILE_OVERSCAN_ROW_COUNT)).toBe(TILE_OVERSCAN_ROW_COUNT);
            expect(computeOverscanRowCount(800, 0, TILE_OVERSCAN_ROW_COUNT)).toBe(TILE_OVERSCAN_ROW_COUNT);
            expect(computeOverscanRowCount(-1, TABLE_ROW_HEIGHT, TABLE_OVERSCAN_ROW_COUNT)).toBe(TABLE_OVERSCAN_ROW_COUNT);
        });

        it('keeps the floor when the viewport is small', () => {
            expect(computeOverscanRowCount(400, TILE_ROW_STRIDE, TILE_OVERSCAN_ROW_COUNT)).toBe(TILE_OVERSCAN_ROW_COUNT);
            expect(computeOverscanRowCount(400, TABLE_ROW_HEIGHT, TABLE_OVERSCAN_ROW_COUNT)).toBe(TABLE_OVERSCAN_ROW_COUNT);
        });

        it('grows with viewport height for zoomed-out / tall screens', () => {
            const tileOverscan = computeOverscanRowCount(5000, TILE_ROW_STRIDE, TILE_OVERSCAN_ROW_COUNT);
            expect(tileOverscan).toBe(Math.ceil((5000 / TILE_ROW_STRIDE) * OVERSCAN_VIEWPORT_SCREENS));
            expect(tileOverscan).toBeGreaterThan(TILE_OVERSCAN_ROW_COUNT);

            const tableOverscan = computeOverscanRowCount(1800, TABLE_ROW_HEIGHT, TABLE_OVERSCAN_ROW_COUNT);
            expect(tableOverscan).toBe(Math.ceil((1800 / TABLE_ROW_HEIGHT) * OVERSCAN_VIEWPORT_SCREENS));
            expect(tableOverscan).toBeGreaterThan(TABLE_OVERSCAN_ROW_COUNT);
        });
    });

    describe('bidirectionalOverscanIndicesGetter', () => {
        const visible = {startIndex: 20, stopIndex: 30, cellCount: 100, overscanCellsCount: 8, scrollDirection: 1 as const};

        it('overscans equally above and below the visible range', () => {
            expect(bidirectionalOverscanIndicesGetter(visible)).toEqual({
                overscanStartIndex: 12,
                overscanStopIndex: 38
            });
        });

        it('returns the same range regardless of scroll direction (no reversal remounts)', () => {
            const forward = bidirectionalOverscanIndicesGetter({...visible, scrollDirection: 1});
            const backward = bidirectionalOverscanIndicesGetter({...visible, scrollDirection: -1});
            expect(forward).toEqual(backward);
        });

        it('is a no-op when overscanCellsCount is 0 (Grid horizontal default)', () => {
            expect(
                bidirectionalOverscanIndicesGetter({
                    cellCount: 100,
                    overscanCellsCount: 0,
                    scrollDirection: 1,
                    startIndex: 20,
                    stopIndex: 30
                })
            ).toEqual({overscanStartIndex: 20, overscanStopIndex: 30});
        });

        it('clamps to list bounds', () => {
            expect(
                bidirectionalOverscanIndicesGetter({
                    cellCount: 5,
                    overscanCellsCount: 8,
                    scrollDirection: 1,
                    startIndex: 0,
                    stopIndex: 2
                })
            ).toEqual({overscanStartIndex: 0, overscanStopIndex: 4});
        });
    });

    describe('shouldUseVirtualScroll', () => {
        it('requires opt-in and length above the threshold', () => {
            expect(shouldUseVirtualScroll(true, VIRTUAL_THRESHOLD)).toBe(false);
            expect(shouldUseVirtualScroll(true, VIRTUAL_THRESHOLD + 1)).toBe(true);
            expect(shouldUseVirtualScroll(false, VIRTUAL_THRESHOLD + 1)).toBe(false);
            expect(shouldUseVirtualScroll(undefined, VIRTUAL_THRESHOLD + 1)).toBe(false);
        });
    });

    describe('computeColumnsPerRow', () => {
        it('returns 1 column for narrow containers', () => {
            expect(computeColumnsPerRow(400, TILE_MIN_WIDTH, TILE_GAP)).toBe(1);
        });

        it('returns multiple columns for wide containers', () => {
            expect(computeColumnsPerRow(1200, TILE_MIN_WIDTH, TILE_GAP)).toBe(3);
        });
    });

    describe('computeColumnWidth', () => {
        it('accounts for gaps between columns like CSS grid 1fr tracks', () => {
            const width = 1200;
            const columns = computeColumnsPerRow(width, TILE_MIN_WIDTH, TILE_GAP);
            const columnWidth = computeColumnWidth(width, columns, TILE_GAP);
            expect(columns * columnWidth + (columns - 1) * TILE_GAP).toBeLessThanOrEqual(width);
        });
    });

    describe('computeColumnWidthForIndex', () => {
        it('omits the trailing gap on the last column', () => {
            const width = 1200;
            const columns = computeColumnsPerRow(width, TILE_MIN_WIDTH, TILE_GAP);
            const tileWidth = computeColumnWidth(width, columns, TILE_GAP);

            for (let i = 0; i < columns - 1; i++) {
                expect(computeColumnWidthForIndex(width, columns, i, TILE_GAP)).toBe(tileWidth + TILE_GAP);
            }
            expect(computeColumnWidthForIndex(width, columns, columns - 1, TILE_GAP)).toBe(tileWidth);
        });

        it('keeps total Grid content width within the container (no horizontal overflow)', () => {
            for (const width of [400, 800, 1200, 1600, 1920]) {
                const columns = computeColumnsPerRow(width, TILE_MIN_WIDTH, TILE_GAP);
                let total = 0;
                for (let i = 0; i < columns; i++) {
                    total += computeColumnWidthForIndex(width, columns, i, TILE_GAP);
                }
                expect(total).toBeLessThanOrEqual(width);
            }
        });
    });

    describe('getTableRowHeight', () => {
        it('returns the baseline height without a hydrator operation', () => {
            expect(getTableRowHeight(app('demo'))).toBe(TABLE_ROW_HEIGHT);
        });

        it('returns the taller height when a source-hydrator operation is active', () => {
            expect(getTableRowHeight(app('demo', true))).toBe(TABLE_ROW_HEIGHT_WITH_HYDRATOR);
            expect(hasActiveHydrator(app('demo', true))).toBe(true);
        });
    });

    describe('appsLayoutKey', () => {
        it('is stable for the same order and hydrator state', () => {
            const apps = [app('a'), app('b', true)];
            expect(appsLayoutKey(apps)).toBe(appsLayoutKey([...apps]));
        });

        it('changes when apps are reordered', () => {
            const a = app('a');
            const b = app('b', true);
            expect(appsLayoutKey([a, b])).not.toBe(appsLayoutKey([b, a]));
        });

        it('changes when a hydrator flips without length change', () => {
            expect(appsLayoutKey([app('a'), app('b')])).not.toBe(appsLayoutKey([app('a', true), app('b')]));
        });

        it('changes when one hydrator starts and another finishes (count collision)', () => {
            expect(appsLayoutKey([app('a', true), app('b')])).not.toBe(appsLayoutKey([app('a'), app('b', true)]));
        });

        it('changes when Path, Chart, or Last Sync rows appear or disappear', () => {
            const base = app('a');
            const withPath = {
                ...base,
                spec: {...base.spec, source: {path: 'manifests'}}
            } as Application;
            const withChart = {
                ...base,
                spec: {...base.spec, source: {chart: 'nginx'}}
            } as Application;
            const withLastSync = {
                ...base,
                status: {...base.status, operationState: {finishedAt: '2026-01-01T00:00:00Z'}}
            } as Application;

            expect(appsLayoutKey([withPath])).not.toBe(appsLayoutKey([base]));
            expect(appsLayoutKey([withChart])).not.toBe(appsLayoutKey([base]));
            expect(appsLayoutKey([withLastSync])).not.toBe(appsLayoutKey([base]));
            // Text length alone does not change height (ellipsis); presence already covered above.
            expect(appsLayoutKey([withPath])).toBe(
                appsLayoutKey([
                    {
                        ...withPath,
                        spec: {...withPath.spec, source: {path: 'manifests/very/long/path'}}
                    } as Application
                ])
            );
        });
    });

    describe('useWindowScrollerPosition', () => {
        it('updates an enabled scroller when the surrounding layout changes', () => {
            const updatePosition = jest.fn();
            const scrollerRef = {current: {updatePosition}};
            const {rerender} = renderHook(
                ({enabled, layoutKey}: {enabled: boolean; layoutKey: string}) => useWindowScrollerPosition(scrollerRef, enabled, layoutKey),
                {initialProps: {enabled: true, layoutKey: 'header-hidden'}}
            );

            expect(updatePosition).toHaveBeenCalledTimes(1);

            rerender({enabled: true, layoutKey: 'header-hidden'});
            expect(updatePosition).toHaveBeenCalledTimes(1);

            rerender({enabled: true, layoutKey: 'header-visible'});
            expect(updatePosition).toHaveBeenCalledTimes(2);

            rerender({enabled: false, layoutKey: 'header-hidden'});
            expect(updatePosition).toHaveBeenCalledTimes(2);
        });
    });

});
