import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {DataLoader} from '../../../shared/components';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSyncWindowStatusIcon, ComparisonStatusIcon, getAppOperationState, getConditionCategory, HealthStatusIcon, OperationState, syncStatusMessage} from '../utils';
import {RevisionMetadataPanel} from './revision-metadata-panel';

require('./application-status-panel.scss');

interface Props {
    application: models.Application;
    showOperation?: () => any;
    showConditions?: () => any;
    showMetadataInfo?: (revision: string, info: models.RevisionMetadata) => any;
}

const sectionLabel = (title: string, helpContent?: string) => (
    <label>
        {title}
        {helpContent && <HelpIcon title={helpContent} />}
    </label>
);

export const ApplicationStatusPanel = ({application, showOperation, showConditions, showMetadataInfo}: Props) => {
    const today = new Date();

    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }

    application.status.conditions = [
        {
            type: 'Error',
            message: 'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
            lastTransitionTime: 'Wed Jan 20 2021 14:52:45 GMT-0800'
        },
        {
            type: 'Warning',
            message: 'Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat',
            lastTransitionTime: 'Wed Jan 20 2021 14:52:45 GMT-0800'
        },
        {
            type: 'info',
            message: 'Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur',
            lastTransitionTime: 'Wed Jan 20 2021 14:52:45 GMT-0800'
        }
    ];
    const cntByCategory = (application.status.conditions || []).reduce(
        (map, next) => map.set(getConditionCategory(next), (map.get(getConditionCategory(next)) || 0) + 1),
        new Map<string, number>()
    );
    const appOperationState = getAppOperationState(application);
    if (application.metadata.deletionTimestamp) {
        showOperation = null;
    }

    const infos = cntByCategory.get('info');
    const warnings = cntByCategory.get('warning');
    const errors = cntByCategory.get('error');

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                {sectionLabel('APP HEALTH', 'The health status of your app')}
                <div className='application-status-panel__item-value'>
                    <HealthStatusIcon state={application.status.health} />
                    &nbsp;
                    {application.status.health.status}
                </div>
                <div className='application-status-panel__item-name'>{application.status.health.message}</div>
            </div>
            <div className='application-status-panel__item'>
                <div style={{display: 'flex', alignItems: 'center'}}>
                    {sectionLabel(
                        'CURRENT SYNC STATUS',
                        'Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.'
                    )}
                    <button
                        className='argo-button argo-button--base argo-button--sm application-status-panel__more-button'
                        onClick={showMetadataInfo && (info => showMetadataInfo(application.status.sync.revision, info))}>
                        MORE
                    </button>
                </div>
                <div className='application-status-panel__item-value'>
                    <div>
                        <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                    </div>
                    <div className='application-status-panel__item-value__revision'>{syncStatusMessage(application)}</div>
                </div>
                <div className='application-status-panel__item-name'></div>
                <div className='application-status-panel__item-name'>
                    {application.status && application.status.sync && application.status.sync.revision && (
                        <RevisionMetadataPanel
                            appName={application.metadata.name}
                            type={application.spec.source.chart && 'helm'}
                            revision={application.status.sync.revision}
                            showInfo={}
                        />
                    )}
                </div>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item'>
                    {sectionLabel(
                        'LAST SYNC',
                        'Whether or not your last app sync was successful. It has been ' + daysSinceLastSynchronized + ' days since last sync. Click for the status of that sync.'
                    )}
                    <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                        <a onClick={() => showOperation && showOperation()}>
                            <OperationState app={application} />{' '}
                        </a>
                        {appOperationState.syncResult && appOperationState.syncResult.revision && (
                            <div className='application-status-panel__item-value__revision'>
                                To <Revision repoUrl={application.spec.source.repoURL} revision={appOperationState.syncResult.revision} />
                            </div>
                        )}
                    </div>

                    <div className='application-status-panel__item-name'>
                        {appOperationState.phase} <Timestamp date={appOperationState.finishedAt || appOperationState.startedAt} />
                    </div>
                    {(appOperationState.syncResult && appOperationState.syncResult.revision && (
                        <RevisionMetadataPanel
                            appName={application.metadata.name}
                            type={application.spec.source.chart && 'helm'}
                            revision={appOperationState.syncResult.revision}
                            showInfo={showMetadataInfo && (info => showMetadataInfo(appOperationState.syncResult.revision, info))}
                        />
                    )) || <div className='application-status-panel__item-name'>{appOperationState.message}</div>}
                </div>
            )}
            {application.status.conditions && (
                <div className={`application-status-panel__item`}>
                    {sectionLabel('APP CONDITIONS')}
                    <div className='application-status-panel__item-value application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
                        {infos && (
                            <a className='info'>
                                <i className='fa fa-info-circle' /> {infos} Info
                            </a>
                        )}
                        {warnings && (
                            <a className='warning'>
                                <i className='fa fa-exclamation-triangle' /> {warnings} Warning{warnings !== 1 && 's'}
                            </a>
                        )}
                        {errors && (
                            <a className='error'>
                                <i className='fa fa-exclamation-circle' /> {errors} Error{errors !== 1 && 's'}
                            </a>
                        )}
                    </div>
                </div>
            )}
            <DataLoader
                noLoaderOnInputChange={true}
                input={application.metadata.name}
                load={async name => {
                    return await services.applications.getApplicationSyncWindowState(name);
                }}>
                {(data: models.ApplicationSyncWindowState) => (
                    <React.Fragment>
                        <div className='application-status-panel__item columns' style={{position: 'relative'}}>
                            <div className='application-status-panel__item-value'>
                                {data.assignedWindows && (
                                    <React.Fragment>
                                        <ApplicationSyncWindowStatusIcon project={application.spec.project} state={data} />
                                        <HelpIcon
                                            title={
                                                'The aggregate state of sync windows for this app. ' +
                                                'Red: no syncs allowed. ' +
                                                'Yellow: manual syncs allowed. ' +
                                                'Green: all syncs allowed'
                                            }
                                        />
                                    </React.Fragment>
                                )}
                            </div>
                        </div>
                    </React.Fragment>
                )}
            </DataLoader>
        </div>
    );
};
