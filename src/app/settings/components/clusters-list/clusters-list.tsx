import { MockupList } from 'argo-ui';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';

import { ConnectionStateIcon, Page } from '../../../shared/components';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export class ClustersList extends React.Component<RouteComponentProps<any>, { clusters: models.Cluster[] }> {

    constructor(props: RouteComponentProps<any>) {
        super(props);
        this.state = { clusters: null };
    }

    public componentDidMount() {
        this.reloadClusters();
    }

    public render() {
        return (
            <Page title='Clusters' toolbar={{ breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Clusters'}] }}>
                <div className='repos-list'>
                    <div className='argo-container'>
                    {this.state.clusters ? (
                        this.state.clusters.length > 0 && (
                        <div className='argo-table-list'>
                            <div className='argo-table-list__head'>
                                <div className='row'>
                                    <div className='columns small-3'>NAME</div>
                                    <div className='columns small-6'>URL</div>
                                    <div className='columns small-3'>CONNECTION STATUS</div>
                                </div>
                            </div>
                            {this.state.clusters.map((cluster) => (
                                <div className='argo-table-list__row' key={cluster.server}>
                                    <div className='row'>
                                        <div className='columns small-3'>
                                            <i className='icon argo-icon-hosts'/> {cluster.name}
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
                    ) : <MockupList height={50} marginTop={30}/>}
                    </div>
                </div>
            </Page>
        );
    }

    private async reloadClusters() {
        this.setState({ clusters: await services.clustersService.list() });
    }
}
