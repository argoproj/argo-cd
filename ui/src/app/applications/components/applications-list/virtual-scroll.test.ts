import {act, renderHook} from '@testing-library/react';
import {
    appsLayoutKey,
    computeColumnStride,
    computeColumnWidth,
    computeColumnWidthForIndex,
    computeColumnsPerRow,
    computeGridContentWidth,
    getTableRowHeight,
    hasActiveHydrator,
    shouldUseVirtualScroll,
    TABLE_OVERSCAN_ROW_COUNT,
    TABLE_ROW_HEIGHT,
    TABLE_ROW_HEIGHT_WITH_HYDRATOR,
    TILE_GAP,
    TILE_MIN_WIDTH,
    TILE_OVERSCAN_ROW_COUNT,
    useVirtualViewportHeight,
    VIRTUAL_THRESHOLD,
    VIRTUAL_VIEWPORT_BOTTOM_PADDING,
    VIRTUAL_VIEWPORT_MIN_HEIGHT
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
        it('uses enough tile/table overscan to avoid blank flashes on fast scroll', () => {
            expect(TILE_OVERSCAN_ROW_COUNT).toBeGreaterThanOrEqual(6);
            expect(TABLE_OVERSCAN_ROW_COUNT).toBeGreaterThanOrEqual(10);
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

    describe('computeColumnStride', () => {
        it('is content width plus gap', () => {
            const width = 1600;
            const columns = 4;
            expect(computeColumnStride(width, columns, TILE_GAP)).toBe(computeColumnWidth(width, columns, TILE_GAP) + TILE_GAP);
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
                expect(computeGridContentWidth(width, columns, TILE_GAP)).toBeLessThanOrEqual(width);
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

    describe('useVirtualViewportHeight', () => {
        let observe: jest.Mock;
        let disconnect: jest.Mock;
        let rafCallbacks: FrameRequestCallback[];

        beforeEach(() => {
            rafCallbacks = [];
            observe = jest.fn();
            disconnect = jest.fn();
            (global as unknown as {ResizeObserver: unknown}).ResizeObserver = jest.fn(() => ({
                observe,
                unobserve: jest.fn(),
                disconnect
            }));
            jest.spyOn(window, 'requestAnimationFrame').mockImplementation(cb => {
                rafCallbacks.push(cb);
                return rafCallbacks.length;
            });
            jest.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => undefined);
            Object.defineProperty(window, 'innerHeight', {configurable: true, value: 900});
        });

        afterEach(() => {
            jest.restoreAllMocks();
        });

        const flushRaf = () => {
            const pending = [...rafCallbacks];
            rafCallbacks = [];
            pending.forEach(cb => cb(0));
        };

        const attachElement = (top: number) => {
            const parent = document.createElement('div');
            const el = document.createElement('div');
            parent.appendChild(el);
            document.body.appendChild(parent);
            jest.spyOn(el, 'getBoundingClientRect').mockReturnValue({top} as DOMRect);
            return {el, parent};
        };

        it('does not measure or observe when disabled', () => {
            const {result} = renderHook(() => useVirtualViewportHeight(false));
            const {el, parent} = attachElement(100);

            act(() => {
                result.current[1](el);
            });

            expect(result.current[0]).toBe(VIRTUAL_VIEWPORT_MIN_HEIGHT);
            expect(observe).not.toHaveBeenCalled();
            document.body.removeChild(parent);
        });

        it('measures height from the element top to the window bottom', () => {
            const {result} = renderHook(() => useVirtualViewportHeight(true));
            const {el, parent} = attachElement(100);

            act(() => {
                result.current[1](el);
                flushRaf();
            });

            expect(result.current[0]).toBe(900 - 100 - VIRTUAL_VIEWPORT_BOTTOM_PADDING);
            expect(observe).toHaveBeenCalledWith(parent);
            document.body.removeChild(parent);
        });

        it('clamps to the minimum when available space is small', () => {
            const {result} = renderHook(() => useVirtualViewportHeight(true));
            const {el, parent} = attachElement(850);

            act(() => {
                result.current[1](el);
                flushRaf();
            });

            expect(result.current[0]).toBe(VIRTUAL_VIEWPORT_MIN_HEIGHT);
            document.body.removeChild(parent);
        });

        it('recomputes on window resize and cleans up on unmount', () => {
            const removeSpy = jest.spyOn(window, 'removeEventListener');
            const {result, unmount} = renderHook(() => useVirtualViewportHeight(true));
            const {el, parent} = attachElement(200);

            act(() => {
                result.current[1](el);
                flushRaf();
            });
            expect(result.current[0]).toBe(900 - 200 - VIRTUAL_VIEWPORT_BOTTOM_PADDING);

            jest.spyOn(el, 'getBoundingClientRect').mockReturnValue({top: 50} as DOMRect);
            act(() => {
                window.dispatchEvent(new Event('resize'));
                flushRaf();
            });
            expect(result.current[0]).toBe(900 - 50 - VIRTUAL_VIEWPORT_BOTTOM_PADDING);

            unmount();
            expect(disconnect).toHaveBeenCalled();
            expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function));
            expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function));
            document.body.removeChild(parent);
        });
    });
});
