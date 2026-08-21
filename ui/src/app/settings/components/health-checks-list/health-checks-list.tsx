import {Tooltip} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {DataLoader, EmptyState, FlexTopBar, IconColumn, Page, Paginate, SearchBar} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {useListSort} from '../../../shared/hooks/use-list-sort';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {HealthChecksFilter, HealthChecksListPreferences, filterHealthChecks, getHealthCheckFilterResults} from './health-checks-filter';
import {HealthCheckDetailsPanel} from './health-check-details-panel';

import './health-checks-list.scss';

export const renderOriginBadge = (origin: string) => {
    let modifier = '';
    switch (origin) {
        case 'BuiltinGo':
            modifier = 'health-checks-list__origin--builtin-go';
            break;
        case 'BuiltinLua':
            modifier = 'health-checks-list__origin--builtin-lua';
            break;
        case 'CustomLua':
            modifier = 'health-checks-list__origin--custom-lua';
            break;
        case 'OverrideLua':
            modifier = 'health-checks-list__origin--override-lua';
            break;
    }

    return <span className={`health-checks-list__origin ${modifier}`}>{origin}</span>;
};

export function searchHealthChecks(items: models.HealthCheckItem[], searchText: string): models.HealthCheckItem[] {
    const searchLower = (searchText || '').trim().toLowerCase();
    if (!searchLower) {
        return items;
    }
    return items.filter(
        item =>
            (item.group && item.group.toLowerCase().includes(searchLower)) ||
            (item.kind && item.kind.toLowerCase().includes(searchLower)) ||
            (item.key && item.key.toLowerCase().includes(searchLower))
    );
}

export function sortHealthChecks(
    items: models.HealthCheckItem[],
    sortKey: 'group' | 'kind' | 'key' | 'origin',
    compareString: (a: string, b: string) => number
): models.HealthCheckItem[] {
    return [...items].sort((a, b) => {
        switch (sortKey) {
            case 'group':
                return compareString(a.group || '', b.group || '') || compareString(a.kind || '', b.kind || '') || compareString(a.key, b.key);
            case 'kind':
                return compareString(a.kind || '', b.kind || '') || compareString(a.group || '', b.group || '') || compareString(a.key, b.key);
            case 'key':
                return compareString(a.key, b.key);
            case 'origin':
                return compareString(a.origin, b.origin) || compareString(a.key, b.key);
            default:
                return compareString(a.group || '', b.group || '') || compareString(a.kind || '', b.kind || '') || compareString(a.key, b.key);
        }
    });
}

export const HealthChecksList = () => {
    const ctx = React.useContext(Context);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);
    const sidebarTarget = useSidebarTarget();
    const loaderRef = React.useRef<DataLoader | null>(null);

    const originParams = query.getAll('origin') || [];
    const originKey = originParams.join(',');

    const filterPref: HealthChecksListPreferences = React.useMemo(() => {
        return {
            originFilter: originParams
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [originKey]);

    type SortKey = 'group' | 'kind' | 'key' | 'origin';
    const {sortKey, requestSort, sortIcon, compareString} = useListSort<SortKey>('group');

    const updateFilterPref = (newPref: HealthChecksListPreferences) => {
        ctx.navigation.goto(
            '.',
            {
                search: searchText || null,
                origin: newPref.originFilter.length > 0 ? newPref.originFilter : null,
                key: query.get('key') || null
            },
            {replace: true}
        );
        setPage(0);
    };

    return (
        <Page title='Health Checks' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Health Checks'}]}}>
            <FlexTopBar
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Health Checks'}],

                    tools: (
                        <SearchBar
                            value={searchText}
                            onChange={value => {
                                ctx.navigation.goto(
                                    '.',
                                    {
                                        search: value || null,
                                        origin: filterPref.originFilter.length > 0 ? filterPref.originFilter : null,
                                        key: query.get('key') || null
                                    },
                                    {replace: true}
                                );
                                setPage(0);
                            }}
                            placeholder='Search health checks...'
                        />
                    )
                }}
            />
            <div className='argo-container health-checks-list'>
                <DataLoader ref={loaderRef} load={() => services.authService.healthChecks()}>
                    {(healthChecks: models.HealthCheckItem[]) => {
                        const filterResults = getHealthCheckFilterResults(healthChecks || [], filterPref);
                        const filteredByOrigin = filterHealthChecks(filterResults);
                        const filteredItems = searchHealthChecks(filteredByOrigin, searchText);
                        const sortedItems = sortHealthChecks(filteredItems, sortKey, compareString);

                        const selectedKey = query.get('key');
                        const selectedItem = (healthChecks || []).find(hc => hc.key === selectedKey) || null;

                        return (
                            <>
                                {ReactDOM.createPortal(
                                    <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                        {allpref => <HealthChecksFilter items={filterResults} pref={filterPref} onChange={updateFilterPref} collapsed={allpref.hideSidebar} />}
                                    </DataLoader>,
                                    sidebarTarget?.current
                                )}

                                {sortedItems.length > 0 ? (
                                    <Paginate
                                        page={page}
                                        data={sortedItems}
                                        onPageChange={setPage}
                                        emptyState={() => <EmptyState icon='fa fa-heartbeat'>No health checks found</EmptyState>}>
                                        {pageItems => (
                                            <div className='argo-table-list argo-table-list--clickable'>
                                                <div className='argo-table-list__head'>
                                                    <div className='row'>
                                                        <div className='columns small-2' onClick={() => requestSort('group')}>
                                                            GROUP {sortIcon('group')}
                                                        </div>
                                                        <div className='columns small-3' onClick={() => requestSort('kind')}>
                                                            KIND {sortIcon('kind')}
                                                        </div>
                                                        <div className='columns small-4' onClick={() => requestSort('key')}>
                                                            KEY {sortIcon('key')}
                                                        </div>
                                                        <div className='columns small-2' onClick={() => requestSort('origin')}>
                                                            ORIGIN {sortIcon('origin')}
                                                        </div>
                                                        <div className='columns small-1'>WILDCARD</div>
                                                    </div>
                                                </div>
                                                {pageItems.map(item => (
                                                    <div
                                                        key={item.key}
                                                        className='argo-table-list__row'
                                                        data-testid={`health-check-row-${item.key}`}
                                                        onClick={() =>
                                                            ctx.navigation.goto(
                                                                '.',
                                                                {
                                                                    search: searchText || null,
                                                                    origin: filterPref.originFilter.length > 0 ? filterPref.originFilter : null,
                                                                    key: item.key
                                                                },
                                                                {replace: true}
                                                            )
                                                        }>
                                                        <div className='row'>
                                                            <IconColumn icon='fa fa-heartbeat' />
                                                            <div className='columns small-2'>
                                                                <Tooltip content={item.group || 'core'}>
                                                                    <span>{item.group || 'core'}</span>
                                                                </Tooltip>
                                                            </div>
                                                            <div className='columns small-3'>
                                                                <Tooltip content={item.kind}>
                                                                    <span>{item.kind}</span>
                                                                </Tooltip>
                                                            </div>
                                                            <div className='columns small-4'>
                                                                <Tooltip content={item.key}>
                                                                    <span>{item.key}</span>
                                                                </Tooltip>
                                                            </div>
                                                            <div className='columns small-2'>{renderOriginBadge(item.origin)}</div>
                                                            <div className='columns small-1'>
                                                                {item.isWildcard ? (
                                                                    <span className='health-checks-list__wildcard-tag' title='Wildcard check'>
                                                                        <i className='fa fa-asterisk' /> Yes
                                                                    </span>
                                                                ) : (
                                                                    'No'
                                                                )}
                                                            </div>
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </Paginate>
                                ) : (
                                    <EmptyState icon='fa fa-heartbeat'>
                                        <h4>No health checks found</h4>
                                        <p>No resource health checks match your search/filter criteria.</p>
                                    </EmptyState>
                                )}

                                <HealthCheckDetailsPanel
                                    item={selectedItem}
                                    onClose={() =>
                                        ctx.navigation.goto(
                                            '.',
                                            {
                                                search: searchText || null,
                                                origin: filterPref.originFilter.length > 0 ? filterPref.originFilter : null,
                                                key: null
                                            },
                                            {replace: true}
                                        )
                                    }
                                />
                            </>
                        );
                    }}
                </DataLoader>
            </div>
        </Page>
    );
};
