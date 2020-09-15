import {DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {Cluster} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {OperationState} from '../utils';
require('./applications-table.scss');

export const ApplicationsTable = (props: {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}) => (
    <Consumer>
        {ctx => (
            <div className='applications-table argo-table-list argo-table-list--clickable'>
                {props.applications.map(app => (
                    <div
                        key={app.metadata.name}
                        className={`argo-table-list__row
                applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                applications-list__entry--health-${app.status.health.status}`}>
                        <div className='row applications-list__table-row' onClick={e => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})}>
                            <div className='columns small-4'>
                                <div className='row'>
                                    <div className='show-for-xxlarge columns small-3'>Project:</div>
                                    <div className='columns small-12 xxlarge-9'>{app.spec.project}</div>
                                </div>
                                <div className='row'>
                                    <div className='show-for-xxlarge columns small-3'>Name:</div>
                                    <div className='columns small-12 xxlarge-9'>
                                        {app.metadata.name} <ApplicationURLs urls={app.status.summary.externalURLs} />
                                    </div>
                                </div>
                            </div>
                            <div className='columns small-6'>
                                <div className='row'>
                                    <div className='show-for-xxlarge columns small-2'>Source:</div>
                                    <div className='columns small-12 xxlarge-10' style={{position: 'relative'}}>
                                        {app.spec.source.repoURL}/{app.spec.source.path || app.spec.source.chart}
                                        <div className='applications-table__meta'>
                                            <span>{app.spec.source.targetRevision || 'HEAD'}</span>
                                            {Object.keys(app.metadata.labels || {}).map(label => (
                                                <span key={label}>{`${label}=${app.metadata.labels[label]}`}</span>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                                <div className='row'>
                                    <div className='show-for-xxlarge columns small-2'>Destination:</div>
                                    <div className='columns small-12 xxlarge-10'>
                                        <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />/{app.spec.destination.namespace}
                                    </div>
                                </div>
                            </div>
                            <div className='columns small-2'>
                                <AppUtils.HealthStatusIcon state={app.status.health} /> <span>{app.status.health.status}</span>
                                <br />
                                <AppUtils.ComparisonStatusIcon status={app.status.sync.status} />
                                <span>{app.status.sync.status}</span> <OperationState app={app} quiet={true} />
                                <DropDownMenu
                                    anchor={() => (
                                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                            <i className='fa fa-ellipsis-v' />
                                        </button>
                                    )}
                                    items={[
                                        {title: 'Sync', action: () => props.syncApplication(app.metadata.name)},
                                        {title: 'Refresh', action: () => props.refreshApplication(app.metadata.name)},
                                        {title: 'Delete', action: () => props.deleteApplication(app.metadata.name)}
                                    ]}
                                />
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        )}
    </Consumer>
);
