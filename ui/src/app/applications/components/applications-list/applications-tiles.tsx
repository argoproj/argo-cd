import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';
import CellMeasurer, {CellMeasurerCache} from 'react-virtualized/dist/commonjs/CellMeasurer';
import Grid from 'react-virtualized/dist/commonjs/Grid';
import type {GridCellProps} from 'react-virtualized';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {isApp} from '../utils';
import {services} from '../../../shared/services';
import {ApplicationTile} from './application-tile';
import {AppSetTile} from './appset-tile';
import {
    appsLayoutKey,
    computeColumnWidth,
    computeColumnWidthForIndex,
    computeColumnsPerRow,
    shouldUseVirtualScroll,
    TILE_GAP,
    TILE_HEIGHT,
    TILE_MIN_WIDTH,
    TILE_OVERSCAN_ROW_COUNT,
    useVirtualViewportHeight
} from './virtual-scroll';

import './applications-tiles.scss';

export interface ApplicationTilesProps {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
    useVirtualScrolling?: boolean;
}

const useItemsPerContainer = (itemRef: React.RefObject<HTMLDivElement | null>, containerRef: React.RefObject<HTMLElement | null>, enabled: boolean = true): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        if (!enabled) {
            return;
        }
        let timeoutId: ReturnType<typeof setTimeout>;
        const handleResize = () => {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                setItemsPer(prev => (curItemsPer !== prev ? curItemsPer : prev));
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            clearTimeout(timeoutId);
            window.removeEventListener('resize', handleResize);
        };
    }, [itemRef, containerRef, enabled]);

    return itemsPer || 1;
};

const VirtualizedTilesGrid = ({
    applications,
    cellCache,
    getRowHeight,
    gridRef,
    onLayoutWidth,
    renderTile
}: {
    applications: models.AbstractApplication[];
    cellCache: CellMeasurerCache;
    getRowHeight: (params: {index: number}) => number;
    gridRef: React.RefObject<Grid | null>;
    onLayoutWidth: (width: number) => void;
    renderTile: (app: models.AbstractApplication, index: number) => React.ReactNode;
}) => (
    <AutoSizer onResize={({width}) => onLayoutWidth(width)}>
        {({height, width}) => {
            const columnsPerRow = computeColumnsPerRow(width);
            const rowCount = Math.ceil(applications.length / columnsPerRow);
            const tileWidth = computeColumnWidth(width, columnsPerRow);

            const cellRenderer = ({columnIndex, key, parent, rowIndex, style}: GridCellProps) => {
                const index = rowIndex * columnsPerRow + columnIndex;
                if (index >= applications.length) {
                    return null;
                }

                const app = applications[index];
                // Tile is content-width; the Grid column already includes the gap.
                const cellStyle: React.CSSProperties = {
                    ...style,
                    width: tileWidth
                };

                return (
                    <CellMeasurer cache={cellCache} columnIndex={columnIndex} key={key} parent={parent} rowIndex={rowIndex}>
                        <div style={cellStyle} className='applications-tiles__virtual-cell'>
                            {renderTile(app, index)}
                        </div>
                    </CellMeasurer>
                );
            };

            return (
                <div role='grid' aria-rowcount={rowCount} aria-colcount={columnsPerRow} style={{height, width}}>
                    <Grid
                        ref={gridRef}
                        deferredMeasurementCache={cellCache}
                        height={height}
                        width={width}
                        columnCount={columnsPerRow}
                        columnWidth={({index}) => computeColumnWidthForIndex(width, columnsPerRow, index)}
                        rowCount={rowCount}
                        rowHeight={getRowHeight}
                        cellRenderer={cellRenderer}
                        overscanRowCount={TILE_OVERSCAN_ROW_COUNT}
                        scrollingResetTimeInterval={150}
                    />
                </div>
            );
        }}
    </AutoSizer>
);

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication, useVirtualScrolling}: ApplicationTilesProps) => {
    const [selectedApp, navApp, reset] = useNav(applications.length);

    const ctxh = React.useContext(Context);
    const firstTileRef = React.useRef<HTMLDivElement>(null);
    const appContainerRef = React.useRef<HTMLDivElement>(null);
    const gridRef = React.useRef<Grid>(null);
    const [layoutWidth, setLayoutWidth] = React.useState(0);

    const shouldVirtualize = shouldUseVirtualScroll(useVirtualScrolling, applications.length);
    const [viewportHeight, viewportRef] = useVirtualViewportHeight(shouldVirtualize);
    const appsPerRow = useItemsPerContainer(firstTileRef, appContainerRef, !shouldVirtualize);
    const columnsPerRow = layoutWidth > 0 ? computeColumnsPerRow(layoutWidth) : 1;
    const layoutKey = React.useMemo(() => (shouldVirtualize ? appsLayoutKey(applications) : ''), [shouldVirtualize, applications]);

    const [cellCache] = React.useState(
        () =>
            new CellMeasurerCache({
                defaultHeight: TILE_HEIGHT,
                defaultWidth: TILE_MIN_WIDTH,
                fixedWidth: true,
                minHeight: 1
            })
    );

    const {registerKeybinding} = React.useContext(KeybindingContext);
    const verticalNavStep = shouldVirtualize ? columnsPerRow : appsPerRow;

    registerKeybinding({keys: Key.RIGHT, action: () => navApp(1)});
    registerKeybinding({keys: Key.LEFT, action: () => navApp(-1)});
    registerKeybinding({
        keys: Key.DOWN,
        action: () => navApp(verticalNavStep)
    });
    registerKeybinding({
        keys: Key.UP,
        action: () => navApp(-1 * verticalNavStep)
    });

    registerKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    registerKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    registerKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    registerKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });

    React.useEffect(() => {
        if (selectedApp >= applications.length) {
            reset();
        }
    }, [selectedApp, applications.length, reset]);

    React.useEffect(() => {
        if (selectedApp < 0 || !shouldVirtualize || !gridRef.current) {
            return;
        }
        gridRef.current.scrollToCell({
            columnIndex: selectedApp % columnsPerRow,
            rowIndex: Math.floor(selectedApp / columnsPerRow)
        });
    }, [selectedApp, shouldVirtualize, columnsPerRow]);

    // Remeasure after sort/reorder, hydrator change, or column-width change.
    React.useEffect(() => {
        if (!shouldVirtualize) {
            return;
        }
        cellCache.clearAll();
        gridRef.current?.recomputeGridSize();
    }, [shouldVirtualize, cellCache, layoutKey, layoutWidth]);

    const getRowHeight = React.useCallback(
        ({index}: {index: number}) => {
            const height = cellCache.rowHeight({index});
            const lastRow = Math.max(0, Math.ceil(applications.length / columnsPerRow) - 1);
            return index >= lastRow ? height : height + TILE_GAP;
        },
        [cellCache, applications.length, columnsPerRow]
    );

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const renderTile = (app: models.AbstractApplication, i: number, tileRef?: React.RefObject<HTMLDivElement>) =>
                            isApp(app) ? (
                                <ApplicationTile
                                    key={AppUtils.appInstanceName(app)}
                                    app={app as models.Application}
                                    selected={selectedApp === i}
                                    pref={pref}
                                    ctx={ctx}
                                    tileRef={tileRef}
                                    syncApplication={syncApplication}
                                    refreshApplication={refreshApplication}
                                    deleteApplication={deleteApplication}
                                />
                            ) : (
                                <AppSetTile
                                    key={AppUtils.appInstanceName(app)}
                                    appSet={app as models.ApplicationSet}
                                    selected={selectedApp === i}
                                    pref={pref}
                                    ctx={ctx}
                                    tileRef={tileRef}
                                />
                            );

                        if (shouldVirtualize) {
                            return (
                                <div
                                    ref={viewportRef}
                                    className='applications-list__virtual-viewport applications-tiles applications-tiles--virtualized argo-table-list argo-table-list--clickable'
                                    style={{height: viewportHeight}}>
                                    <VirtualizedTilesGrid
                                        applications={applications}
                                        cellCache={cellCache}
                                        getRowHeight={getRowHeight}
                                        gridRef={gridRef}
                                        onLayoutWidth={width => setLayoutWidth(prev => (prev !== width ? width : prev))}
                                        renderTile={renderTile}
                                    />
                                </div>
                            );
                        }

                        return (
                            <div className='applications-tiles argo-table-list argo-table-list--clickable' ref={appContainerRef}>
                                {applications.map((app, i) => renderTile(app, i, i === 0 ? firstTileRef : undefined))}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
