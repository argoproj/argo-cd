import * as classNames from 'classnames';
import * as moment from 'moment';
import * as React from 'react';
import {FieldApi, FormField as ReactFormField, Text} from 'react-form';
import {RouteComponentProps} from 'react-router-dom';
import {from, timer} from 'rxjs';
import {mergeMap} from 'rxjs/operators';

import {FormField, Ticker} from 'argo-ui';
import {ConnectionStateIcon, DataLoader, EditablePanel, Page, Timestamp, MapInputField} from '../../../shared/components';
import {Cluster} from '../../../shared/models';
import {services} from '../../../shared/services';

function isRefreshRequested(cluster: Cluster): boolean {
    return cluster.info.connectionState.attemptedAt && cluster.refreshRequestedAt && moment(cluster.info.connectionState.attemptedAt).isBefore(moment(cluster.refreshRequestedAt));
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
        <DataLoader ref={loaderRef} input={server} load={(url: string) => timer(0, 1000).pipe(mergeMap(() => from(services.clusters.get(url, ''))))}>
            {(cluster: Cluster) => (
                <Page
                    title='Clusters'
                    toolbar={{
                        breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Clusters', path: '/settings/clusters'}, {title: server}],
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
                                const item = await services.clusters.get(updated.server, '');
                                item.name = updated.name;
                                item.namespaces = updated.namespaces;
                                item.labels = updated.labels;
                                item.annotations = updated.annotations;
                                loaderRef.current.setData(await services.clusters.update(item, 'name', 'namespaces', 'labels', 'annotations'));
                            }}
                            title='GENERAL'
                            items={[
                                {
                                    title: 'SERVER',
                                    view: cluster.server
                                },
                                {
                                    title: 'CREDENTIALS TYPE',
                                    view:
                                        (cluster.config.awsAuthConfig && `IAM AUTH (cluster name: ${cluster.config.awsAuthConfig.clusterName})`) ||
                                        (cluster.config.execProviderConfig && `External provider (command: ${cluster.config.execProviderConfig.command})`) ||
                                        'Token/Basic Auth'
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
                                },
                                {
                                    title: 'LABELS',
                                    view: Object.keys(cluster.labels || [])
                                        .map(label => `${label}=${cluster.labels[label]}`)
                                        .join(' '),
                                    edit: formApi => <FormField formApi={formApi} field='labels' component={MapInputField} />
                                },
                                {
                                    title: 'ANNOTATIONS',
                                    view: Object.keys(cluster.annotations || [])
                                        .map(annotation => `${annotation}=${cluster.annotations[annotation]}`)
                                        .join(' '),
                                    edit: formApi => <FormField formApi={formApi} field='annotations' component={MapInputField} />
                                }
                            ]}
                        />
                        <div className='white-box'>
                            <p>CONNECTION STATE</p>
                            <div className='white-box__details'>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>STATUS:</div>
                                    <div className='columns small-9'>
                                        <ConnectionStateIcon state={cluster.info.connectionState} /> {cluster.info.connectionState.status}
                                    </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>VERSION:</div>
                                    <div className='columns small-9'> {cluster.info.serverVersion}</div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>DETAILS:</div>
                                    <div className='columns small-9'> {cluster.info.connectionState.message} </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>MODIFIED AT:</div>
                                    <div className='columns small-9'>
                                        <Ticker>
                                            {now => {
                                                if (!cluster.info.connectionState.attemptedAt) {
                                                    return <span>Never (next refresh in few seconds)</span>;
                                                }
                                                const secondsBeforeRefresh = Math.round(Math.max(10 - now.diff(moment(cluster.info.connectionState.attemptedAt)) / 1000, 1));
                                                return (
                                                    <React.Fragment>
                                                        <Timestamp date={cluster.info.connectionState.attemptedAt} /> (next refresh in {secondsBeforeRefresh} seconds)
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
                                                <Timestamp date={cluster.info.cacheInfo.lastCacheSyncTime} />
                                            </div>
                                        </div>
                                    )}
                                </Ticker>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>APIs COUNT:</div>
                                    <div className='columns small-9'> {cluster.info.cacheInfo.apisCount} </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>RESOURCES COUNT:</div>
                                    <div className='columns small-9'> {cluster.info.cacheInfo.resourcesCount} </div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>APPLICATIONS COUNT:</div>
                                    <div className='columns small-9'> {cluster.info.applicationsCount} </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </Page>
            )}
        </DataLoader>
    );
};
