import {DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {RouteComponentProps} from 'react-router-dom';
import {clusterName, ConnectionStateIcon, DataLoader, EmptyState, Page} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

export const ClustersList = (props: RouteComponentProps<{}>) => {
    const clustersLoaderRef = React.useRef<DataLoader>();
    return (
        <Consumer>
            {ctx => (
                <React.Fragment>
                    <Page title='Clusters' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Clusters'}]}}>
                        <div className='repos-list'>
                            <div className='argo-container'>
                                <DataLoader
                                    ref={clustersLoaderRef}
                                    load={() => services.clusters.list().then(clusters => clusters.sort((first, second) => first.name.localeCompare(second.name)))}>
                                    {(clusters: models.Cluster[]) =>
                                        (clusters.length > 0 && (
                                            <div className='argo-table-list argo-table-list--clickable'>
                                                <div className='argo-table-list__head'>
                                                    <div className='row'>
                                                        <div className='columns small-3'>NAME</div>
                                                        <div className='columns small-5'>URL</div>
                                                        <div className='columns small-2'>VERSION</div>
                                                        <div className='columns small-2'>CONNECTION STATUS</div>
                                                    </div>
                                                </div>
                                                {clusters.map(cluster => (
                                                    <div
                                                        className='argo-table-list__row'
                                                        key={cluster.server}
                                                        onClick={() => ctx.navigation.goto(`./${encodeURIComponent(cluster.server)}`)}>
                                                        <div className='row'>
                                                            <div className='columns small-3'>
                                                                <i className='icon argo-icon-hosts' /> {clusterName(cluster.name)}
                                                            </div>
                                                            <div className='columns small-5'>{cluster.server}</div>
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
                                                                            action: () =>
                                                                                services.clusters.delete(cluster.server).finally(() => {
                                                                                    ctx.navigation.goto('.', {new: null});
                                                                                    if (clustersLoaderRef.current) {
                                                                                        clustersLoaderRef.current.reload();
                                                                                    }
                                                                                })
                                                                        }
                                                                    ]}
                                                                />
                                                            </div>
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        )) || (
                                            <EmptyState icon='argo-icon-hosts'>
                                                <h4>No clusters connected</h4>
                                                <h5>Connect more clusters using argocd CLI</h5>
                                            </EmptyState>
                                        )
                                    }
                                </DataLoader>
                            </div>
                        </div>
                    </Page>
                </React.Fragment>
            )}
        </Consumer>
    );
};
