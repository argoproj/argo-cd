import {DropDownMenu, ErrorNotification, NotificationType, Tooltip} from 'argo-ui';
import * as React from 'react';
import {clusterName, ConnectionStateIcon, DataLoader, EmptyState, Page, Paginate, SearchBar} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {FlexTopBar} from '../../../shared/components';

import './cluster-list.scss';


export const ClustersList = () => {
    const clustersLoaderRef = React.useRef<DataLoader | null>(null);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);

    return (
        <Consumer>
            {ctx => (
                <Page title='Clusters' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Clusters'}]}} hideAuth={true}>
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
                    <div className='repos-list'>
                        <div className='argo-container'>
                            <DataLoader
                                ref={clustersLoaderRef}
                                load={() => services.clusters.list().then(clusters => clusters.sort((first, second) => first.name.localeCompare(second.name)))}>
                                {(clusters: models.Cluster[]) => {
                                    const filteredClusters = clusters.filter(
                                        cluster =>
                                            searchText === '' ||
                                            clusterName(cluster.name).toLowerCase().includes(searchText.toLowerCase()) ||
                                            cluster.server.toLowerCase().includes(searchText.toLowerCase())
                                    );

                                    return (
                                        <>
                                            {filteredClusters.length > 0 ? (
                                                <Paginate page={page} data={filteredClusters} onPageChange={setPage} preferencesKey='clusters-list'>
                                                    {clustersToDisplay => (
                                                        <div className='argo-table-list argo-table-list--clickable'>
                                                            <div className='argo-table-list__head'>
                                                                <div className='row'>
                                                                    <div className='columns small-3'>NAME</div>
                                                                    <div className='columns small-5'>URL</div>
                                                                    <div className='columns small-2'>VERSION</div>
                                                                    <div className='columns small-2'>CONNECTION STATUS</div>
                                                                </div>
                                                            </div>
                                                            {clustersToDisplay.map(cluster => (
                                                                <div
                                                                    className='argo-table-list__row'
                                                                    key={cluster.server}
                                                                    onClick={() => ctx.navigation.goto(`./${encodeURIComponent(cluster.server)}`)}>
                                                                    <div className='row'>
                                                                        <div className='columns small-3'>
                                                                            <i className='icon argo-icon-hosts' />
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
                                                                            <DropDownMenu
                                                                                anchor={() => (
                                                                                    <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                                        <i className='fa fa-ellipsis-v' />
                                                                                    </button>
                                                                                )}
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
                                                <EmptyState icon='argo-icon-hosts'>
                                                    <h4>No clusters matched your search</h4>
                                                    <h5>Try adjusting your search query</h5>
                                                </EmptyState>
                                            )}
                                        </>
                                    );
                                }}
                            </DataLoader>
                        </div>
                    </div>
                </Page>
            )}
        </Consumer>
    );
};
