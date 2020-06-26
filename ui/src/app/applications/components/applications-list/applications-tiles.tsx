import {Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';

import {Cluster} from '../../../shared/components';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {OperationState} from '../utils';

require('./applications-tiles.scss');

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication}: ApplicationTilesProps) => (
    <Consumer>
        {ctx => (
            <div className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4'>
                {applications.map(app => (
                    <div key={app.metadata.name} className='column column-block'>
                        <div
                            className={`argo-table-list__row
                    applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                    applications-list__entry--health-${app.status.health.status}`}>
                            <div className='row' onClick={e => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})}>
                                <div className='columns small-12 applications-list__info'>
                                    <div className='applications-list__external-link'>
                                        <ApplicationURLs urls={app.status.summary.externalURLs} />
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-12'>
                                            <i className={'icon argo-icon-' + (app.spec.source.chart != null ? 'helm' : 'git')} />
                                            <span className='applications-list__title'>{app.metadata.name}</span>
                                        </div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Project:'>
                                            Project:
                                        </div>
                                        <div className='columns small-9'>{app.spec.project}</div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Labels:'>
                                            Labels:
                                        </div>
                                        <div className='columns small-9'>
                                            <Tooltip
                                                content={
                                                    <div>
                                                        {Object.keys(app.metadata.labels || {})
                                                            .map(label => ({label, value: app.metadata.labels[label]}))
                                                            .map(item => (
                                                                <div key={item.label}>
                                                                    {item.label}={item.value}
                                                                </div>
                                                            ))}
                                                    </div>
                                                }>
                                                <span>
                                                    {Object.keys(app.metadata.labels || {})
                                                        .map(label => `${label}=${app.metadata.labels[label]}`)
                                                        .join(', ')}
                                                </span>
                                            </Tooltip>
                                        </div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Status:'>
                                            Status:
                                        </div>
                                        <div className='columns small-9' qe-id='applications-tiles-health-status'>
                                            <AppUtils.HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                                            &nbsp;
                                            <AppUtils.ComparisonStatusIcon status={app.status.sync.status} /> {app.status.sync.status}
                                            &nbsp;
                                            <OperationState app={app} quiet={true} />
                                        </div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Repository:'>
                                            Repository:
                                        </div>
                                        <div className='columns small-9'>
                                            <Tooltip content={app.spec.source.repoURL}>
                                                <span>{app.spec.source.repoURL}</span>
                                            </Tooltip>
                                        </div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Target Revision:'>
                                            Target Revision:
                                        </div>
                                        <div className='columns small-9'>{app.spec.source.targetRevision}</div>
                                    </div>
                                    {app.spec.source.path && (
                                        <div className='row'>
                                            <div className='columns small-3' title='Path:'>
                                                Path:
                                            </div>
                                            <div className='columns small-9'>{app.spec.source.path}</div>
                                        </div>
                                    )}
                                    {app.spec.source.chart && (
                                        <div className='row'>
                                            <div className='columns small-3' title='Chart:'>
                                                Chart:
                                            </div>
                                            <div className='columns small-9'>{app.spec.source.chart}</div>
                                        </div>
                                    )}
                                    <div className='row'>
                                        <div className='columns small-3' title='Destination:'>
                                            Destination:
                                        </div>
                                        <div className='columns small-9'>
                                            <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />
                                        </div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns small-3' title='Namespace:'>
                                            Namespace:
                                        </div>
                                        <div className='columns small-9'>{app.spec.destination.namespace}</div>
                                    </div>
                                    <div className='row'>
                                        <div className='columns applications-list__entry--actions'>
                                            <a
                                                className='argo-button argo-button--base'
                                                qe-id='applications-tiles-button-sync'
                                                onClick={e => {
                                                    e.stopPropagation();
                                                    syncApplication(app.metadata.name);
                                                }}>
                                                <i className='fa fa-sync' /> Sync
                                            </a>
                                            &nbsp;
                                            <a
                                                className='argo-button argo-button--base'
                                                qe-id='applications-tiles-button-refresh'
                                                {...AppUtils.refreshLinkAttrs(app)}
                                                onClick={e => {
                                                    e.stopPropagation();
                                                    refreshApplication(app.metadata.name);
                                                }}>
                                                <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(app)})} />{' '}
                                                <span className='show-for-xlarge'>Refresh</span>
                                            </a>
                                            &nbsp;
                                            <a
                                                className='argo-button argo-button--base'
                                                qe-id='applications-tiles-button-delete'
                                                onClick={e => {
                                                    e.stopPropagation();
                                                    deleteApplication(app.metadata.name);
                                                }}>
                                                <i className='fa fa-times-circle' /> Delete
                                            </a>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        )}
    </Consumer>
);
