import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';
import List from 'react-virtualized/dist/commonjs/List';
import WindowScroller from 'react-virtualized/dist/commonjs/WindowScroller';
import type {ListRowProps} from 'react-virtualized';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {isApp} from '../utils';
import {services} from '../../../shared/services';
import {ApplicationTableRow} from './application-table-row';
import {AppSetTableRow} from './appset-table-row';
import {ProjectGroupHeading} from './project-group-heading';
import {buildProjectRows, ProjectGrouping, ProjectRow, useResetSelectionOnIdentityChange, visibleApps} from './project-groups';
import {
    appsLayoutKey,
    bidirectionalOverscanIndicesGetter,
    computeOverscanRowCount,
    getTableRowHeight,
    PROJECT_HEADING_ROW_HEIGHT,
    shouldUseVirtualScroll,
    TABLE_OVERSCAN_ROW_COUNT,
    TABLE_ROW_HEIGHT,
    useWindowScrollerPosition
} from './virtual-scroll';

import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
    useVirtualScrolling?: boolean;
    statusBarVisible?: boolean;
    grouping?: ProjectGrouping;
}) => {
    const rows = React.useMemo(() => buildProjectRows(props.applications, props.grouping), [props.applications, props.grouping]);
    const apps = React.useMemo(() => visibleApps(rows), [rows]);
    const [selectedApp, navApp, reset] = useNav(apps.length);
    useResetSelectionOnIdentityChange(apps, selectedApp, reset);
    const ctxh = React.useContext(Context);
    const listRef = React.useRef<List>(null);
    const windowScrollerRef = React.useRef<WindowScroller>(null);
    const shouldVirtualize = shouldUseVirtualScroll(props.useVirtualScrolling, props.applications.length);

    const {registerKeybinding} = React.useContext(KeybindingContext);

    registerKeybinding({keys: Key.DOWN, action: () => navApp(1)});
    registerKeybinding({keys: Key.UP, action: () => navApp(-1)});
    registerKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedApp > -1 ? true : false;
        }
    });
    registerKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(apps[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    React.useEffect(() => {
        if (selectedApp >= apps.length) {
            reset();
        }
    }, [selectedApp, apps.length, reset]);

    const rowsRef = React.useRef(rows);
    React.useEffect(() => {
        rowsRef.current = rows;
    });

    React.useEffect(() => {
        if (selectedApp >= 0 && shouldVirtualize && listRef.current) {
            listRef.current.scrollToRow(rowsRef.current.findIndex(row => row.kind === 'app' && row.index === selectedApp));
        }
    }, [selectedApp, shouldVirtualize]);

    const getRowHeight = React.useCallback(
        ({index}: {index: number}) => {
            const row = rows[index];
            if (!row) {
                return TABLE_ROW_HEIGHT;
            }
            return row.kind === 'heading' ? PROJECT_HEADING_ROW_HEIGHT : getTableRowHeight(row.app);
        },
        [rows]
    );

    const layoutKey = React.useMemo(() => (shouldVirtualize ? `${appsLayoutKey(apps)}:${rows.length}` : ''), [shouldVirtualize, apps, rows.length]);
    useWindowScrollerPosition(windowScrollerRef, shouldVirtualize, `${layoutKey}:${!!props.statusBarVisible}`);

    // Recalculate row heights after sort/reorder or when a hydrator status line appears/disappears.
    React.useEffect(() => {
        if (shouldVirtualize && listRef.current) {
            listRef.current.recomputeRowHeights();
        }
    }, [shouldVirtualize, layoutKey]);

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const renderApp = (app: models.AbstractApplication, i: number) =>
                            isApp(app) ? (
                                <ApplicationTableRow
                                    key={AppUtils.appInstanceName(app)}
                                    app={app as models.Application}
                                    selected={selectedApp === i}
                                    pref={pref}
                                    ctx={ctx}
                                    syncApplication={props.syncApplication}
                                    refreshApplication={props.refreshApplication}
                                    deleteApplication={props.deleteApplication}
                                />
                            ) : (
                                <AppSetTableRow key={AppUtils.appInstanceName(app)} appSet={app as models.ApplicationSet} selected={selectedApp === i} pref={pref} ctx={ctx} />
                            );

                        const renderRow = (row: ProjectRow) =>
                            row.kind === 'heading' ? (
                                <ProjectGroupHeading
                                    key={`project-${row.project}`}
                                    project={row.project}
                                    count={row.count}
                                    collapsed={row.collapsed}
                                    onToggle={() => props.grouping.onToggle(row.project)}
                                />
                            ) : (
                                renderApp(row.app, row.index)
                            );

                        if (shouldVirtualize) {
                            const rowRenderer = ({index, key, style}: ListRowProps) => {
                                const row = rows[index];
                                if (!row) {
                                    return null;
                                }
                                return (
                                    <div key={key} style={style} className='applications-table__virtual-row'>
                                        {renderRow(row)}
                                    </div>
                                );
                            };

                            return (
                                <div className='applications-table argo-table-list argo-table-list--clickable'>
                                    <WindowScroller ref={windowScrollerRef} updateScrollTopOnUpdatePosition={true}>
                                        {({height, isScrolling, onChildScroll, scrollTop}) => (
                                            <AutoSizer disableHeight={true}>
                                                {({width}) => (
                                                    <List
                                                        ref={listRef}
                                                        autoHeight={true}
                                                        height={height}
                                                        width={width}
                                                        isScrolling={isScrolling}
                                                        onScroll={onChildScroll}
                                                        scrollTop={scrollTop}
                                                        rowCount={rows.length}
                                                        rowHeight={getRowHeight}
                                                        rowRenderer={rowRenderer}
                                                        overscanRowCount={computeOverscanRowCount(height, TABLE_ROW_HEIGHT, TABLE_OVERSCAN_ROW_COUNT)}
                                                        overscanIndicesGetter={bidirectionalOverscanIndicesGetter}
                                                    />
                                                )}
                                            </AutoSizer>
                                        )}
                                    </WindowScroller>
                                </div>
                            );
                        }

                        return <div className='applications-table argo-table-list argo-table-list--clickable'>{rows.map(renderRow)}</div>;
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
