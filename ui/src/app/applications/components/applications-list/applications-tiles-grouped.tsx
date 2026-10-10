import * as React from 'react';
import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';
import CellMeasurer, {CellMeasurerCache} from 'react-virtualized/dist/commonjs/CellMeasurer';
import List from 'react-virtualized/dist/commonjs/List';
import WindowScroller from 'react-virtualized/dist/commonjs/WindowScroller';
import type {ListRowProps} from 'react-virtualized';
import * as models from '../../../shared/models';
import {buildTileRows, ProjectRow} from './project-groups';
import {ProjectGroupHeading} from './project-group-heading';
import {
    bidirectionalOverscanIndicesGetter,
    computeColumnsPerRow,
    computeOverscanRowCount,
    TILE_OVERSCAN_ROW_COUNT,
    TILE_ROW_STRIDE,
    useWindowScrollerPosition
} from './virtual-scroll';

export interface VirtualizedGroupedTilesProps {
    rows: ProjectRow[];
    selectedApp: number;
    layoutKey: string;
    renderTile: (app: models.AbstractApplication, index: number) => React.ReactNode;
    onToggle: (project: string) => void;
}

// Each virtual row is a project heading or one grid row of tiles from a single project.
export const VirtualizedGroupedTiles = ({rows, selectedApp, layoutKey, renderTile, onToggle}: VirtualizedGroupedTilesProps) => {
    const listRef = React.useRef<List>(null);
    const windowScrollerRef = React.useRef<WindowScroller>(null);
    const [layoutWidth, setLayoutWidth] = React.useState(0);
    const [cache] = React.useState(() => new CellMeasurerCache({defaultHeight: TILE_ROW_STRIDE, fixedWidth: true, minHeight: 1}));

    useWindowScrollerPosition(windowScrollerRef, true, layoutKey);

    React.useEffect(() => {
        cache.clearAll();
        listRef.current?.recomputeRowHeights();
    }, [cache, layoutKey, layoutWidth]);

    const rowsRef = React.useRef(rows);
    React.useEffect(() => {
        rowsRef.current = rows;
    });

    React.useEffect(() => {
        if (selectedApp < 0 || layoutWidth <= 0) {
            return;
        }
        const tileRows = buildTileRows(rowsRef.current, computeColumnsPerRow(layoutWidth));
        const rowIndex = tileRows.findIndex(row => row.kind === 'tiles' && row.items.some(item => item.index === selectedApp));
        if (rowIndex >= 0) {
            listRef.current?.scrollToRow(rowIndex);
        }
    }, [selectedApp, layoutWidth]);

    return (
        <WindowScroller ref={windowScrollerRef} updateScrollTopOnUpdatePosition={true}>
            {({height, isScrolling, onChildScroll, scrollTop}) => (
                <AutoSizer disableHeight={true} onResize={({width}) => setLayoutWidth(prev => (prev !== width ? width : prev))}>
                    {({width}) => {
                        const columnsPerRow = computeColumnsPerRow(width);
                        const tileRows = buildTileRows(rows, columnsPerRow);

                        const rowRenderer = ({index, key, parent, style}: ListRowProps) => {
                            const row = tileRows[index];
                            if (!row) {
                                return null;
                            }
                            return (
                                <CellMeasurer cache={cache} columnIndex={0} key={key} parent={parent} rowIndex={index}>
                                    {row.kind === 'heading' ? (
                                        <div style={style} className='applications-tiles__virtual-group-row applications-tiles__virtual-group-row--heading'>
                                            <ProjectGroupHeading project={row.project} count={row.count} collapsed={row.collapsed} onToggle={() => onToggle(row.project)} />
                                        </div>
                                    ) : (
                                        <div style={style} className='applications-tiles__virtual-group-row'>
                                            <div className='applications-tiles__virtual-group-tiles' style={{gridTemplateColumns: `repeat(${columnsPerRow}, minmax(0, 1fr))`}}>
                                                {row.items.map(item => (
                                                    <div key={item.index} className='applications-tiles__virtual-cell'>
                                                        <div className='applications-tiles__virtual-content' style={{height: '100%'}}>
                                                            {renderTile(item.app, item.index)}
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                </CellMeasurer>
                            );
                        };

                        return (
                            <List
                                ref={listRef}
                                autoHeight={true}
                                height={height}
                                width={width}
                                isScrolling={isScrolling}
                                onScroll={onChildScroll}
                                scrollTop={scrollTop}
                                deferredMeasurementCache={cache}
                                rowCount={tileRows.length}
                                rowHeight={cache.rowHeight}
                                rowRenderer={rowRenderer}
                                overscanRowCount={computeOverscanRowCount(height, TILE_ROW_STRIDE, TILE_OVERSCAN_ROW_COUNT)}
                                overscanIndicesGetter={bidirectionalOverscanIndicesGetter}
                            />
                        );
                    }}
                </AutoSizer>
            )}
        </WindowScroller>
    );
};
