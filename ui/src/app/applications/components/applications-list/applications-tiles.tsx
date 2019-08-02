import { Tooltip } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';

import { ApplicationURLs } from '../application-urls';
import * as AppUtils from '../utils';

require('./applications-tiles.scss');

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication}: ApplicationTilesProps) => (
    <Consumer>
    {(ctx) => (
    <div className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4'>
        {applications.map((app) => (
            <div key={app.metadata.name} className='column column-block'>
                <div className={`argo-table-list__row
                    applications-list__entry applications-list__entry--comparison-${app.status.sync.status}
                    applications-list__entry--health-${app.status.health.status}`
                }>
                    <div className='row' onClick={(e) => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, { event: e })}>
                        <div className='columns small-12 applications-list__info'>
                            <div className='applications-list__external-link'>
                                <ApplicationURLs urls={app.status.summary.externalURLs}/>
                            </div>
                            <div className='row'>
                                <div className='columns applications-list__title'>{app.metadata.name}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Project:</div>
                                <div className='columns small-9'>{app.spec.project}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Status:</div>
                                <div className='columns small-9'>
                                    <AppUtils.HealthStatusIcon state={app.status.health}/> {app.status.health.status}
                                    &nbsp;
                                    <AppUtils.ComparisonStatusIcon status={app.status.sync.status}/> {app.status.sync.status}
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Repository:</div>
                                <div className='columns small-9'>
                                    <Tooltip content={app.spec.source.repoURL}><span>{app.spec.source.repoURL}</span></Tooltip>
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Target Revision:</div>
                                <div className='columns small-9'>{app.spec.source.targetRevision || 'HEAD'}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Path:</div>
                                <div className='columns small-9'>{app.spec.source.path}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Destination:</div>
                                <div className='columns small-9'>{app.spec.destination.server}</div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Namespace:</div>
                                <div className='columns small-9'>
                                    {app.spec.destination.namespace}
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns applications-list__entry--actions'>
                                    <a className='argo-button argo-button--base'
                                        onClick={(e) => {
                                            e.stopPropagation();
                                            syncApplication(app.metadata.name);
                                        }}><i className='fa fa-sync'/> Sync</a>
                                    &nbsp;
                                    <a className='argo-button argo-button--base' {...AppUtils.refreshLinkAttrs(app)}
                                       onClick={(e) => {
                                           e.stopPropagation();
                                           refreshApplication(app.metadata.name);
                                       }}><i className={classNames('fa fa-redo', { 'status-icon--spin': AppUtils.isAppRefreshing(app) })}/> <span className='show-for-xlarge'>
                                           Refresh</span></a>
                                    &nbsp;
                                    <a className='argo-button argo-button--base' onClick={(e) => {
                                        e.stopPropagation();
                                        deleteApplication(app.metadata.name);
                                    }}><i className='fa fa-times-circle'/> Delete</a>
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
