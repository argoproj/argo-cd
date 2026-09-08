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
import {appsLayoutKey, getTableRowHeight, shouldUseVirtualScroll, TABLE_OVERSCAN_ROW_COUNT, TABLE_ROW_HEIGHT, useWindowScrollerPosition} from './virtual-scroll';

import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
    useVirtualScrolling?: boolean;
    statusBarVisible?: boolean;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);
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
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(props.applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    React.useEffect(() => {
        if (selectedApp >= props.applications.length) {
            reset();
        }
    }, [selectedApp, props.applications.length, reset]);

    React.useEffect(() => {
        if (selectedApp >= 0 && shouldVirtualize && listRef.current) {
            listRef.current.scrollToRow(selectedApp);
        }
    }, [selectedApp, shouldVirtualize]);

    const getRowHeight = React.useCallback(
        ({index}: {index: number}) => {
            const app = props.applications[index];
            return app ? getTableRowHeight(app) : TABLE_ROW_HEIGHT;
        },
        [props.applications]
    );

    const layoutKey = React.useMemo(() => (shouldVirtualize ? appsLayoutKey(props.applications) : ''), [shouldVirtualize, props.applications]);
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
                        const renderRow = (app: models.AbstractApplication, i: number) =>
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

                        if (shouldVirtualize) {
                            const rowRenderer = ({index, key, style}: ListRowProps) => {
                                const app = props.applications[index];
                                if (!app) {
                                    return null;
                                }
                                return (
                                    <div key={key} style={style} className='applications-table__virtual-row'>
                                        {renderRow(app, index)}
                                    </div>
                                );
                            };

                            return (
                                <div className='applications-table argo-table-list argo-table-list--clickable' role='list'>
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
                                                        rowCount={props.applications.length}
                                                        rowHeight={getRowHeight}
                                                        rowRenderer={rowRenderer}
                                                        overscanRowCount={TABLE_OVERSCAN_ROW_COUNT}
                                                        scrollingResetTimeInterval={150}
                                                    />
                                                )}
                                            </AutoSizer>
                                        )}
                                    </WindowScroller>
                                </div>
                            );
                        }

                        return <div className='applications-table argo-table-list argo-table-list--clickable'>{props.applications.map((app, i) => renderRow(app, i))}</div>;
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
