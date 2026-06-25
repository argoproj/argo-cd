import {ErrorNotification, NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {ActionMenu, clusterName, ConnectionStateIcon, DataLoader, EmptyState, IconColumn, Page, Paginate, SearchBar} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {useListSort} from '../../../shared/hooks/use-list-sort';
import {FlexTopBar} from '../../../shared/components';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {filterClusters, getClusterFilterResults, ClustersFilter, ClustersListPreferences, ClustersListPreferencesHelper} from './clusters-filter';

import './cluster-list.scss';

// Server versions are normalized server-side to "v<major>.<minor>.<patch>", but we compare them in a
// semver-aware way to be defensive against any suffix (e.g. "-gke.100", "-rc.1", "+k3s1") that may slip
// through. The numeric release components are compared numerically (so v1.9.0 sorts before v1.10.0), build
// metadata after "+" is ignored, and a pre-release suffix after "-" ranks below the plain release.
const comparePreRelease = (a: string, b: string): number => {
    // No pre-release outranks any pre-release (e.g. v1.0.0 > v1.0.0-rc.1).
    if (!a && !b) {
        return 0;
    }
    if (!a) {
        return 1;
    }
    if (!b) {
        return -1;
    }
    const ai = a.split('.');
    const bi = b.split('.');
    const len = Math.max(ai.length, bi.length);
    for (let i = 0; i < len; i++) {
        // A larger set of pre-release fields has higher precedence when all preceding fields are equal.
        if (ai[i] === undefined) {
            return -1;
        }
        if (bi[i] === undefined) {
            return 1;
        }
        const aNum = /^\d+$/.test(ai[i]);
        const bNum = /^\d+$/.test(bi[i]);
        if (aNum && bNum) {
            const diff = parseInt(ai[i], 10) - parseInt(bi[i], 10);
            if (diff !== 0) {
                return diff;
            }
        } else if (aNum) {
            // Numeric identifiers always have lower precedence than alphanumeric ones.
            return -1;
        } else if (bNum) {
            return 1;
        } else {
            const diff = ai[i].localeCompare(bi[i]);
            if (diff !== 0) {
                return diff;
            }
        }
    }
    return 0;
};

export const compareServerVersion = (a: string, b: string): number => {
    const split = (v: string) => {
        // Strip leading "v" and drop build metadata (everything after "+"), then separate the pre-release
        // suffix (everything after the first "-") from the numeric release components.
        const cleaned = (v || '').replace(/^v/i, '').split('+')[0];
        const dashIdx = cleaned.indexOf('-');
        const release = dashIdx === -1 ? cleaned : cleaned.slice(0, dashIdx);
        const pre = dashIdx === -1 ? '' : cleaned.slice(dashIdx + 1);
        return {
            release: release.split('.').map(p => parseInt(p, 10)),
            pre
        };
    };
    const sa = split(a);
    const sb = split(b);

    const len = Math.max(sa.release.length, sb.release.length);
    for (let i = 0; i < len; i++) {
        const na = isNaN(sa.release[i]) ? 0 : sa.release[i];
        const nb = isNaN(sb.release[i]) ? 0 : sb.release[i];
        if (na !== nb) {
            return na - nb;
        }
    }

    return comparePreRelease(sa.pre, sb.pre);
};

export const ClustersList = () => {
    const clustersLoaderRef = React.useRef<DataLoader | null>(null);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);
    const [filterPref, setFilterPref] = React.useState<ClustersListPreferences>({
        statusFilter: [],
        credentialFilter: []
    });
    const sidebarTarget = useSidebarTarget();

    type SortKey = 'name' | 'url' | 'version';
    const {sortKey, dir, requestSort, sortIcon, compareString} = useListSort<SortKey>('name');

    const onFilterChange = (newPref: ClustersListPreferences) => {
        setFilterPref(newPref);
        setPage(0);
    };

    return (
        <Consumer>
            {ctx => (
                <Page title='Clusters' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Clusters'}]}}>
                    <FlexTopBar
                        toolbar={{
                            breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Clusters'}],
                            tools: (
                                <SearchBar
                                    value={searchText}
                                    onChange={value => {
                                        ctx.navigation.goto('.', {search: value || null}, {replace: true});
                                        setPage(0);
                                    }}
                                    placeholder='Search clusters...'
                                />
                            )
                        }}
                    />
                    <div className='clusters-list__info-banner'>
                        <i className='fa fa-info-circle' />
                        <span>
                            Refer to CLI{' '}
                            <a href='https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-management/#adding-a-cluster' target='_blank' rel='noopener noreferrer'>
                                <i className='fa fa-external-link-alt' /> Documentation
                            </a>{' '}
                            for adding clusters.
                        </span>
                    </div>
                    <div className='clusters-list__banner-spacer' />
                    <div className='argo-container'>
                        <DataLoader ref={clustersLoaderRef} load={() => services.clusters.list()}>
                            {(clusters: models.Cluster[]) => {
                                const filterResults = getClusterFilterResults(clusters, filterPref);
                                const filteredByStatus = filterClusters(filterResults);
                                const filteredClusters = filteredByStatus
                                    .filter(
                                        cluster =>
                                            searchText === '' ||
                                            clusterName(cluster.name).toLowerCase().includes(searchText.toLowerCase()) ||
                                            cluster.server.toLowerCase().includes(searchText.toLowerCase())
                                    )
                                    .sort((a, b) => {
                                        switch (sortKey) {
                                            case 'name':
                                                return compareString(clusterName(a.name), clusterName(b.name));
                                            case 'url':
                                                return compareString(a.server, b.server);
                                            case 'version':
                                                return dir * compareServerVersion(a.info.serverVersion, b.info.serverVersion);
                                            default:
                                                return 0;
                                        }
                                    });

                                return (
                                    <>
                                        {ReactDOM.createPortal(
                                            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                {allpref => <ClustersFilter clusters={filterResults} pref={filterPref} onChange={onFilterChange} collapsed={allpref.hideSidebar} />}
                                            </DataLoader>,
                                            sidebarTarget?.current
                                        )}
                                        {filteredClusters.length > 0 ? (
                                            <Paginate page={page} data={filteredClusters} onPageChange={setPage} preferencesKey='clusters-list'>
                                                {clustersToDisplay => (
                                                    <div className='argo-table-list argo-table-list--clickable'>
                                                        <div className='argo-table-list__head'>
                                                            <div className='row'>
                                                                <IconColumn />
                                                                <div className='columns small-2 sortable' onClick={() => requestSort('name')}>
                                                                    NAME
                                                                    {sortIcon('name')}
                                                                </div>
                                                                <div className='columns small-5 sortable' onClick={() => requestSort('url')}>
                                                                    URL
                                                                    {sortIcon('url')}
                                                                </div>
                                                                <div className='columns small-2 sortable' onClick={() => requestSort('version')}>
                                                                    VERSION
                                                                    {sortIcon('version')}
                                                                </div>
                                                                <div className='columns small-2'>CONNECTION STATUS</div>
                                                            </div>
                                                        </div>
                                                        {clustersToDisplay.map(cluster => (
                                                            <div
                                                                className='argo-table-list__row'
                                                                key={cluster.server}
                                                                onClick={() => ctx.navigation.goto(`./${encodeURIComponent(cluster.server)}`)}>
                                                                <div className='row'>
                                                                    <IconColumn icon='argo-icon-hosts' />
                                                                    <div className='columns small-2'>
                                                                        <Tooltip content={clusterName(cluster.name)}>
                                                                            <span>{clusterName(cluster.name)}</span>
                                                                        </Tooltip>
                                                                    </div>
                                                                    <div className='columns small-5'>
                                                                        <Tooltip content={cluster.server}>
                                                                            <span>{cluster.server}</span>
                                                                        </Tooltip>
                                                                    </div>
                                                                    <div className='columns small-2'>{cluster.info.serverVersion}</div>
                                                                    <div className='columns small-2'>
                                                                        <ConnectionStateIcon state={cluster.info.connectionState} /> {cluster.info.connectionState.status}
                                                                        <ActionMenu
                                                                            items={[
                                                                                {
                                                                                    title: 'Delete',
                                                                                    action: async () => {
                                                                                        const confirmed = await ctx.popup.confirm(
                                                                                            'Delete cluster?',
                                                                                            `Are you sure you want to delete cluster: ${cluster.name}`
                                                                                        );
                                                                                        if (confirmed) {
                                                                                            try {
                                                                                                await services.clusters.delete(cluster.server).finally(() => {
                                                                                                    ctx.navigation.goto('.', {new: null}, {replace: true});
                                                                                                    if (clustersLoaderRef.current) {
                                                                                                        clustersLoaderRef.current.reload();
                                                                                                    }
                                                                                                });
                                                                                            } catch (e) {
                                                                                                ctx.notifications.show({
                                                                                                    content: <ErrorNotification title='Unable to delete cluster' e={e} />,
                                                                                                    type: NotificationType.Error
                                                                                                });
                                                                                            }
                                                                                        }
                                                                                    }
                                                                                }
                                                                            ]}
                                                                        />
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        ))}
                                                    </div>
                                                )}
                                            </Paginate>
                                        ) : clusters.length === 0 ? (
                                            <EmptyState icon='argo-icon-hosts'>
                                                <h4>No clusters connected</h4>
                                                <h5>Connect more clusters using argocd CLI</h5>
                                            </EmptyState>
                                        ) : (
                                            <EmptyState icon='fa fa-search'>
                                                <h4>No matching clusters found</h4>
                                                <h5>
                                                    Change filter criteria or&nbsp;
                                                    <a
                                                        onClick={() => {
                                                            const newPref = {...filterPref};
                                                            ClustersListPreferencesHelper.clearFilters(newPref);
                                                            onFilterChange(newPref);
                                                        }}>
                                                        clear filters
                                                    </a>
                                                </h5>
                                            </EmptyState>
                                        )}
                                    </>
                                );
                            }}
                        </DataLoader>
                    </div>
                </Page>
            )}
        </Consumer>
    );
};
