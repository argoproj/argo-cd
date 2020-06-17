import * as classNames from 'classnames';
import * as moment from 'moment';
import * as React from 'react';
import {FieldApi, FormField as ReactFormField, Text} from 'react-form';
import {RouteComponentProps} from 'react-router-dom';
import {Observable} from 'rxjs';

import {FormField, Ticker} from 'argo-ui';
import {ConnectionStateIcon, DataLoader, EditablePanel, Page, Timestamp} from '../../../shared/components';
import {Cluster} from '../../../shared/models';
import {services} from '../../../shared/services';

function isRefreshRequested(cluster: Cluster): boolean {
    return (
        cluster.connectionState.attemptedAt &&
        cluster.cacheInfo.refreshRequestedAt &&
        moment(cluster.connectionState.attemptedAt).isBefore(moment(cluster.cacheInfo.refreshRequestedAt))
    );
}

export const NamespacesEditor = ReactFormField((props: {fieldApi: FieldApi; className: string}) => {
    const val = (props.fieldApi.getValue() || []).join(',');
    return <input className={props.className} value={val} onChange={event => props.fieldApi.setValue(event.target.value.split(','))} />;
});

export const ClusterDetails = (props: RouteComponentProps<{server: string}>) => {
    const server = decodeURIComponent(props.match.params.server);
    const loaderRef = React.useRef<DataLoader>();
    const [updating, setUpdating] = React.useState(false);
    return (
        <DataLoader ref={loaderRef} input={server} load={(url: string) => Observable.interval(1000).flatMap(() => Observable.fromPromise(services.clusters.get(url)))}>
            {(cluster: Cluster) => (
                <Page
                    title={server}
                    toolbar={{
                        breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Cluster', path: '/settings/clusters'}, {title: server}],
                        actionMenu: {
                            items: [
                                {
                                    iconClassName: classNames('fa fa-redo', {'status-icon--spin': isRefreshRequested(cluster)}),
                                    title: 'Invalidate Cache',
                                    disabled: isRefreshRequested(cluster) || updating,
                                    action: async () => {
                                        setUpdating(true);
                                        try {
                                            const updated = await services.clusters.invalidateCache(props.match.params.server);
                                            loaderRef.current.setData(updated);
                                        } finally {
                                            setUpdating(false);
                                        }
                                    }
                                }
                            ]
                        }
                    }}>
                    <p />

                    <div className='argo-container'>
                        <EditablePanel
                            values={cluster}
                            save={async updated => {
                                const item = await services.clusters.get(updated.server);
                                item.name = updated.name;
                                item.namespaces = updated.namespaces;
                                loaderRef.current.setData(await services.clusters.update(item));
                            }}
                            title='GENERAL'
                            items={[
                                {
                                    title: 'SERVER',
                                    view: cluster.server
                                },
                                {
                                    title: 'CREDENTIALS TYPE',
                                    view: (cluster.config.awsAuthConfig && `IAM AUTH (cluster name: ${cluster.config.awsAuthConfig.clusterName})`) || 'Token/Basic Auth'
                                },
                                {
                                    title: 'NAME',
                                    view: cluster.name,
                                    edit: formApi => <FormField formApi={formApi} field='name' component={Text} />
                                },
                                {
                                    title: 'NAMESPACES',
                                    view: ((cluster.namespaces || []).length === 0 && 'All namespaces') || cluster.namespaces.join(', '),
                                    edit: formApi => <FormField formApi={formApi} field='namespaces' component={NamespacesEditor} />
                                }
                            ]}
                        />
                        <div className='white-box'>
                            <p>CONNECTION STATE</p>
                            <div className='white-box__details'>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>STATUS:</div>
                                    <div className='columns small-9'>
                                        <ConnectionStateIcon state={cluster.connectionState} /> {cluster.connectionState.status}
                                    </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>DETAILS:</div>
                                    <div className='columns small-9'> {cluster.connectionState.message} </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>MODIFIED AT:</div>
                                    <div className='columns small-9'>
                                        <Ticker>
                                            {now => {
                                                const secondsBeforeRefresh = Math.round(Math.max(30 - now.diff(moment(cluster.connectionState.attemptedAt)) / 1000, 1));
                                                return (
                                                    <React.Fragment>
                                                        <Timestamp date={cluster.connectionState.attemptedAt} /> (next refresh in {secondsBeforeRefresh} seconds)
                                                    </React.Fragment>
                                                );
                                            }}
                                        </Ticker>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div className='white-box'>
                            <p>CACHE INFO</p>
                            <div className='white-box__details'>
                                <Ticker>
                                    {() => (
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-3'>RE-SYNCHRONIZED:</div>
                                            <div className='columns small-9'>
                                                <Timestamp date={cluster.cacheInfo.lastCacheSyncTime} />
                                            </div>
                                        </div>
                                    )}
                                </Ticker>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>APIs COUNT:</div>
                                    <div className='columns small-9'> {cluster.cacheInfo.apisCount} </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>RESOURCES COUNT:</div>
                                    <div className='columns small-9'> {cluster.cacheInfo.resourcesCount} </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </Page>
            )}
        </DataLoader>
    );
};
