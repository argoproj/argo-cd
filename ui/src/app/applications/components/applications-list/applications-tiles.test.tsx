import * as React from 'react';
import {act, cleanup, render} from '@testing-library/react';
import CellMeasurer, {CellMeasurerCache} from 'react-virtualized/dist/commonjs/CellMeasurer';
import Grid from 'react-virtualized/dist/commonjs/Grid';
import WindowScroller from 'react-virtualized/dist/commonjs/WindowScroller';
import type {CellMeasurerProps, GridProps} from 'react-virtualized';
import type {Application} from '../../../shared/models';
import {VirtualizedTilesGrid} from './applications-tiles';
import {computeColumnWidth, computeColumnsPerRow, TILE_GAP, TILE_HEIGHT} from './virtual-scroll';

jest.mock('./application-tile', () => ({ApplicationTile: () => null}));
jest.mock('./appset-tile', () => ({AppSetTile: () => null}));
jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number}) => React.ReactNode}) => children({width: mockWidth})
}));
jest.mock('react-virtualized/dist/commonjs/WindowScroller', () => ({
    __esModule: true,
    default: ({children}: {children: (props: object) => React.ReactNode}) => children({height: 600, isScrolling: false, onChildScroll: jest.fn(), scrollTop: 0})
}));
jest.mock('react-virtualized/dist/commonjs/Grid', () => ({
    __esModule: true,
    default: (props: GridProps) => mockRenderGrid(props)
}));

let mockWidth = 1200;
const mockMeasures = new Map<string, () => void>();
const mockGridStyles = new Map<string, React.CSSProperties>();
const mockParent = {invalidateCellSizeAfterRender: jest.fn(), recomputeGridSize: jest.fn()};
const mockRenderGrid = jest.fn(({cellRenderer, deferredMeasurementCache, rowCount, columnCount, rowHeight, columnWidth}: GridProps) => {
    const cells: React.ReactNode[] = [];
    let top = 0;
    for (let rowIndex = 0; rowIndex < rowCount; rowIndex++) {
        const height = typeof rowHeight === 'function' ? rowHeight({index: rowIndex}) : rowHeight;
        let left = 0;
        for (let columnIndex = 0; columnIndex < columnCount; columnIndex++) {
            const width = typeof columnWidth === 'function' ? columnWidth({index: columnIndex}) : columnWidth;
            const key = `${rowIndex}-${columnIndex}`;
            const measured = deferredMeasurementCache.has(rowIndex, columnIndex);
            const style: React.CSSProperties = {position: 'absolute', top: measured ? top : 0, left: measured ? left : 0, height: measured ? height : 'auto', width};
            mockGridStyles.set(key, {...style});
            const cell = cellRenderer({columnIndex, rowIndex, key, parent: mockParent as unknown as Grid, style, isScrolling: false, isVisible: true});
            if (React.isValidElement<CellMeasurerProps>(cell)) {
                expect(cell.type).toBe(CellMeasurer);
                const child = cell.props.children as React.ReactElement;
                cells.push(
                    React.cloneElement(cell, {
                        children: ({measure}: {measure: () => void}) => {
                            mockMeasures.set(key, measure);
                            return child;
                        }
                    })
                );
            }
            left += width;
        }
        top += height;
    }
    return <div>{cells}</div>;
});

describe('VirtualizedTilesGrid with real CellMeasurer and CellMeasurerCache', () => {
    let intrinsicReads: jest.Mock;

    beforeEach(() => {
        mockWidth = 1200;
        mockMeasures.clear();
        mockGridStyles.clear();
        jest.clearAllMocks();
        intrinsicReads = jest.fn();
        jest.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function (this: HTMLElement) {
            expect(this.className).toBe('applications-tiles__virtual-cell');
            expect(this.style.height).toBe('auto');
            const tile = this.querySelector<HTMLElement>('[data-intrinsic-height]');
            expect(tile).not.toBeNull();
            const height = Number(tile.dataset.intrinsicHeight);
            intrinsicReads(tile.dataset.testid, height);
            return height;
        });
        jest.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(function (this: HTMLElement) {
            expect(this.style.height).toBe('auto');
            return Number.parseFloat(this.style.width);
        });
    });

    afterEach(() => {
        cleanup();
        jest.restoreAllMocks();
    });

    const setup = (heights = [240, 310, 275, 290]) => {
        const applications = heights.map((height, index) => ({kind: 'Application', metadata: {name: `app-${index}`, namespace: 'argocd'}}) as Application);
        const columns = computeColumnsPerRow(mockWidth);
        const rowCount = Math.ceil(applications.length / columns);
        const cache = new CellMeasurerCache({defaultHeight: TILE_HEIGHT, defaultWidth: computeColumnWidth(mockWidth, columns), fixedWidth: true, minHeight: 1});
        const gridRef = React.createRef<Grid>();
        const windowScrollerRef = React.createRef<WindowScroller>();
        const renderGrid = () => (
            <VirtualizedTilesGrid
                applications={applications}
                cellCache={cache}
                getRowHeight={({index}) => cache.rowHeight({index}) + (index < rowCount - 1 ? TILE_GAP : 0)}
                gridRef={gridRef}
                windowScrollerRef={windowScrollerRef}
                onLayoutWidth={jest.fn()}
                renderTile={(app, index) => (
                    <div className='argo-table-list__row' data-testid={app.metadata.name} data-intrinsic-height={heights[index]}>
                        {app.metadata.name}
                    </div>
                )}
            />
        );
        return {cache, columns, rowCount, heights, renderGrid, ...render(renderGrid())};
    };

    const expectCacheHeights = (cache: CellMeasurerCache, columns: number, heights: number[]) => {
        heights.forEach((height, index) => {
            const rowIndex = Math.floor(index / columns);
            const columnIndex = index % columns;
            expect(cache.has(rowIndex, columnIndex)).toBe(true);
            expect(cache.getHeight(rowIndex, columnIndex)).toBe(height);
        });
        for (let rowIndex = 0; rowIndex < Math.ceil(heights.length / columns); rowIndex++) {
            const cachedColumnHeights = Array.from({length: Math.min(columns, heights.length)}, (_, columnIndex) => heights[rowIndex * columns + columnIndex] ?? TILE_HEIGHT);
            expect(cache.rowHeight({index: rowIndex})).toBe(Math.max(...cachedColumnHeights));
        }
    };

    const expectCellStyles = (container: HTMLElement, columns: number, count: number, measured: boolean) => {
        const cells = container.querySelectorAll<HTMLElement>('.applications-tiles__virtual-cell');
        expect(cells).toHaveLength(count);
        cells.forEach((cell, index) => {
            const rowIndex = Math.floor(index / columns);
            const gridStyle = mockGridStyles.get(`${rowIndex}-${index % columns}`);
            expect(cell.style.height).toBe(measured ? `${gridStyle.height}px` : 'auto');
            expect(cell.style.position).toBe(gridStyle.position);
            expect(cell.style.top).toBe(`${gridStyle.top}px`);
            expect(cell.style.left).toBe(`${gridStyle.left}px`);
            expect(cell.style.width).toBe(`${computeColumnWidth(mockWidth, columns)}px`);
            expect(cell.style.marginBottom).toBe('');
            expect(cell.style.paddingBottom).toBe('');
            expect(cell.children).toHaveLength(1);
            const content = cell.firstElementChild as HTMLElement;
            expect(content.className).toBe('applications-tiles__virtual-content');
            expect(content.style.height).toBe(rowIndex < Math.ceil(count / columns) - 1 ? `calc(100% - ${TILE_GAP}px)` : '100%');
            expect(content.firstElementChild).toHaveClass('argo-table-list__row');
        });
    };

    it.each([
        {width: 400, heights: [240]},
        {width: 800, heights: [240, 310, 275, 290]},
        {width: 1200, heights: [240, 310, 275, 290]}
    ])('preserves auto and measured outer heights, with the gap only inside ($width px, $heights)', ({width, heights}) => {
        mockWidth = width;
        const fixture = setup(heights);
        expect(intrinsicReads).toHaveBeenCalledTimes(heights.length);
        expectCellStyles(fixture.container, fixture.columns, heights.length, false);
        expectCacheHeights(fixture.cache, fixture.columns, heights);
        expect(mockParent.invalidateCellSizeAfterRender).toHaveBeenCalledTimes(heights.length);

        fixture.rerender(fixture.renderGrid());

        expectCellStyles(fixture.container, fixture.columns, heights.length, true);
        expectCacheHeights(fixture.cache, fixture.columns, heights);
        expect(intrinsicReads).toHaveBeenCalledTimes(heights.length);
        expect(fixture.getByRole('grid')).toHaveAttribute('aria-rowcount', String(fixture.rowCount));
        expect(fixture.getByRole('grid')).toHaveAttribute('aria-colcount', String(fixture.columns));
        expect(mockMeasures.size).toBe(heights.length);
        expect(mockRenderGrid.mock.calls.every(([props]) => props.deferredMeasurementCache === fixture.cache)).toBe(true);
        if (heights.length % fixture.columns) {
            expect(fixture.cache.has(fixture.rowCount - 1, fixture.columns - 1)).toBe(false);
        }
    });

    it('resets height to auto on every explicit measure, restores outer styles, and keeps both row caches stable', () => {
        const fixture = setup();
        fixture.rerender(fixture.renderGrid());
        const cacheSet = jest.spyOn(fixture.cache, 'set');

        for (let pass = 0; pass < 3; pass++) {
            act(() => mockMeasures.forEach(measure => measure()));
            expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
            expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
            fixture.rerender(fixture.renderGrid());
            expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
        }

        expect(intrinsicReads).toHaveBeenCalledTimes(fixture.heights.length * 4);
        expect(cacheSet).not.toHaveBeenCalled();
        expect(mockParent.recomputeGridSize).not.toHaveBeenCalled();
        expect(mockParent.invalidateCellSizeAfterRender).toHaveBeenCalledTimes(fixture.heights.length);
    });

    it('reuses the same measured cache on remount without another intrinsic read', () => {
        const fixture = setup();
        fixture.rerender(fixture.renderGrid());
        fixture.unmount();

        const remounted = render(fixture.renderGrid());

        expectCellStyles(remounted.container, fixture.columns, fixture.heights.length, true);
        expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
        expect(intrinsicReads).toHaveBeenCalledTimes(fixture.heights.length);
        expect(mockParent.invalidateCellSizeAfterRender).toHaveBeenCalledTimes(fixture.heights.length);
    });

    it('remeasures intrinsic heights after clearAll, then restores measured row slots without moving the gap outside', () => {
        const fixture = setup();
        fixture.rerender(fixture.renderGrid());
        fixture.heights[1] = 350;
        fixture.heights[3] = 220;
        fixture.cache.clearAll();
        expect(fixture.cache.has(0, 0)).toBe(false);
        expect(fixture.cache.has(1, 0)).toBe(false);

        fixture.rerender(fixture.renderGrid());

        expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, false);
        expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
        expect(intrinsicReads).toHaveBeenCalledTimes(fixture.heights.length * 2);
        fixture.rerender(fixture.renderGrid());
        expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
        act(() => mockMeasures.forEach(measure => measure()));
        expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
        expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
        expect(mockParent.recomputeGridSize).not.toHaveBeenCalled();
    });

    it('updates changed intrinsic content through measure and does not repeatedly invalidate unchanged dimensions', () => {
        const fixture = setup();
        fixture.rerender(fixture.renderGrid());
        fixture.heights[1] = 350;
        fixture.heights[3] = 220;
        fixture.rerender(fixture.renderGrid());
        expect(fixture.cache.getHeight(0, 1)).toBe(310);
        expect(fixture.cache.getHeight(1, 0)).toBe(290);

        act(() => mockMeasures.forEach(measure => measure()));

        expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
        expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
        expect(mockParent.recomputeGridSize.mock.calls).toEqual([[{columnIndex: 1, rowIndex: 0}], [{columnIndex: 0, rowIndex: 1}]]);
        fixture.rerender(fixture.renderGrid());
        expectCellStyles(fixture.container, fixture.columns, fixture.heights.length, true);
        act(() => mockMeasures.forEach(measure => measure()));
        expectCacheHeights(fixture.cache, fixture.columns, fixture.heights);
        expect(mockParent.recomputeGridSize).toHaveBeenCalledTimes(2);
    });
});
