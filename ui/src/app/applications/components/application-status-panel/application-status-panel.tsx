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
    showMetadataInfo?: (revision: string) => any;
}

interface SectionInfo {
    title: string;
    helpContent?: string;
}

const sectionLabel = (info: SectionInfo) => (
    <label>
        {info.title}
        {info.helpContent && <HelpIcon title={info.helpContent} />}
    </label>
);

const sectionHeader = (info: SectionInfo, onClick?: () => any) => {
    return (
        <div style={{display: 'flex', alignItems: 'center'}}>
            {sectionLabel(info)}
            {onClick && (
                <button className='argo-button argo-button--base-o argo-button--sm application-status-panel__more-button' onClick={onClick}>
                    MORE
                </button>
            )}
        </div>
    );
};

export const ApplicationStatusPanel = ({application, showOperation, showConditions, showMetadataInfo}: Props) => {
    const today = new Date();

    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    const cntByCategory = (application.status.conditions || []).reduce(
        (map, next) => map.set(getConditionCategory(next), (map.get(getConditionCategory(next)) || 0) + 1),
        new Map<string, number>()
    );
    const appOperationState = getAppOperationState(application);
    if (application.metadata.deletionTimestamp && !appOperationState) {
        showOperation = null;
    }

    const infos = cntByCategory.get('info');
    const warnings = cntByCategory.get('warning');
    const errors = cntByCategory.get('error');

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                {sectionLabel({title: 'APP HEALTH', helpContent: 'The health status of your app'})}
                <div className='application-status-panel__item-value' style={{margin: 'auto 0'}}>
                    <HealthStatusIcon state={application.status.health} />
                    &nbsp;
                    {application.status.health.status}
                </div>
                {application.status.health.message && <div className='application-status-panel__item-name'>{application.status.health.message}</div>}
            </div>
            <div className='application-status-panel__item'>
                <React.Fragment>
                    {sectionHeader(
                        {
                            title: 'CURRENT SYNC STATUS',
                            helpContent: 'Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.'
                        },
                        application.spec.source.chart ? null : () => showMetadataInfo(application.status.sync ? application.status.sync.revision : '')
                    )}
                    <div className='application-status-panel__item-value'>
                        <div>
                            <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                        </div>
                        <div className='application-status-panel__item-value__revision'>{syncStatusMessage(application)}</div>
                    </div>
                    <div className='application-status-panel__item-name'>
                        {application.status && application.status.sync && application.status.sync.revision && (
                            <RevisionMetadataPanel appName={application.metadata.name} type={application.spec.source.chart && 'helm'} revision={application.status.sync.revision} />
                        )}
                    </div>
                </React.Fragment>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item'>
                    <React.Fragment>
                        {sectionHeader(
                            {
                                title: 'LAST SYNC RESULT',
                                helpContent:
                                    'Whether or not your last app sync was successful. It has been ' +
                                    daysSinceLastSynchronized +
                                    ' days since last sync. Click for the status of that sync.'
                            },
                            application.spec.source.chart ? null : () => showMetadataInfo(appOperationState.syncResult ? appOperationState.syncResult.revision : '')
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
                            />
                        )) || <div className='application-status-panel__item-name'>{appOperationState.message}</div>}
                    </React.Fragment>
                </div>
            )}
            {application.status.conditions && (
                <div className={`application-status-panel__item`}>
                    {sectionLabel({title: 'APP CONDITIONS'})}
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
                        {data.assignedWindows && (
                            <div className='application-status-panel__item' style={{position: 'relative'}}>
                                {sectionLabel({
                                    title: 'SYNC WINDOWS',
                                    helpContent:
                                        'The aggregate state of sync windows for this app. ' +
                                        'Red: no syncs allowed. ' +
                                        'Yellow: manual syncs allowed. ' +
                                        'Green: all syncs allowed'
                                })}
                                <div className='application-status-panel__item-value' style={{margin: 'auto 0'}}>
                                    <ApplicationSyncWindowStatusIcon project={application.spec.project} state={data} />
                                </div>
                            </div>
                        )}
                    </React.Fragment>
                )}
            </DataLoader>
        </div>
    );
};
