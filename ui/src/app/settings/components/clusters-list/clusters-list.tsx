import * as React from 'react';
import {clusterName, ConnectionStateIcon, DataLoader, Page} from '../../../shared/components';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const ClustersList = () => (
    <Page title='Clusters' toolbar={{ breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Clusters'}] }}>
        <div className='repos-list'>
            <div className='argo-container'>
                <DataLoader load={() => services.clusters.list().then((clusters) => clusters
                    .sort((first, second) => first.name.localeCompare(second.name)),
                )}>
                {(clusters: models.Cluster[]) => (
                    clusters.length > 0 && (
                    <div className='argo-table-list'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns small-3'>NAME</div>
                                <div className='columns small-6'>URL</div>
                                <div className='columns small-3'>CONNECTION STATUS</div>
                            </div>
                        </div>
                        {clusters.map((cluster) => (
                            <div className='argo-table-list__row' key={cluster.server}>
                                <div className='row'>
                                    <div className='columns small-3'>
                                        <i className='icon argo-icon-hosts'/> {clusterName(cluster.name)}
                                    </div>
                                    <div className='columns small-6'>
                                        {cluster.server}
                                    </div>
                                    <div className='columns small-3'>
                                        <ConnectionStateIcon state={cluster.connectionState}/> {cluster.connectionState.status}
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div> )
                )}
                </DataLoader>
            </div>
        </div>
    </Page>
);
