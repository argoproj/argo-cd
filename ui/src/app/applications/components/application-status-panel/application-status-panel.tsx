import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ARGO_GRAY6_COLOR, DataLoader} from '../../../shared/components';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSyncWindowStatusIcon, ComparisonStatusIcon, getAppDefaultSource, getAppOperationState} from '../utils';
import {getConditionCategory, HealthStatusIcon, OperationState, syncStatusMessage, helpTip} from '../utils';
import {RevisionMetadataPanel} from './revision-metadata-panel';

import './application-status-panel.scss';

interface Props {
    application: models.Application;
    showDiff?: () => any;
    showOperation?: () => any;
    showConditions?: () => any;
    showMetadataInfo?: (revision: string) => any;
}

interface SectionInfo {
    title: string;
    helpContent?: string;
}

const sectionLabel = (info: SectionInfo) => (
    <label style={{fontSize: '12px', fontWeight: 600, color: ARGO_GRAY6_COLOR}}>
        {info.title}
        {info.helpContent && <HelpIcon title={info.helpContent} />}
    </label>
);

const sectionHeader = (info: SectionInfo, hasMultipleSources: boolean, onClick?: () => any) => {
    return (
        <div style={{display: 'flex', alignItems: 'center', marginBottom: '0.5em'}}>
            {sectionLabel(info)}
            {onClick && (
                <button className='application-status-panel__more-button' onClick={onClick} disabled={hasMultipleSources}>
                    {hasMultipleSources && helpTip('More details are not supported for apps with multiple sources')}
                    <i className='fa fa-ellipsis-h' />
                </button>
            )}
        </div>
    );
};

export const ApplicationStatusPanel = ({application, showDiff, showOperation, showConditions, showMetadataInfo}: Props) => {
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
    const source = getAppDefaultSource(application);
    const hasMultipleSources = application.spec.sources && application.spec.sources.length > 0;
    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                <div style={{marginBottom: '1em'}}>{sectionLabel({title: 'APP HEALTH', helpContent: 'The health status of your app'})}</div>
                <div className='application-status-panel__item-value'>
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
                            title: 'SYNC STATUS',
                            helpContent: 'Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.'
                        },
                        hasMultipleSources,
                        () => showMetadataInfo(application.status.sync ? application.status.sync.revision : '')
                    )}
                    {appOperationState && (
                        <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                            <div>
                                {application.status.sync.status === models.SyncStatuses.OutOfSync ? (
                                    <a onClick={() => showDiff && showDiff()}>
                                        <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                                    </a>
                                ) : (
                                    <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                                )}
                            </div>
                            <div className='application-status-panel__item-value__revision show-for-large'>{syncStatusMessage(application)}</div>
                        </div>
                    )}
                    <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                        {application.spec.syncPolicy?.automated ? 'Auto sync is enabled.' : 'Auto sync is not enabled.'}
                    </div>
                    {application.status && application.status.sync && application.status.sync.revision && !application.spec.source.chart && (
                        <div className='application-status-panel__item-name'>
                            <RevisionMetadataPanel
                                appName={application.metadata.name}
                                appNamespace={application.metadata.namespace}
                                type={source.chart && 'helm'}
                                revision={application.status.sync.revision}
                            />
                        </div>
                    )}
                </React.Fragment>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item'>
                    <React.Fragment>
                        {sectionHeader(
                            {
                                title: 'LAST SYNC',
                                helpContent:
                                    'Whether or not your last app sync was successful. It has been ' +
                                    daysSinceLastSynchronized +
                                    ' days since last sync. Click for the status of that sync.'
                            },
                            hasMultipleSources,
                            () => showMetadataInfo(appOperationState.syncResult ? appOperationState.syncResult.revision : '')
                        )}
                        <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                            <a onClick={() => showOperation && showOperation()}>
                                <OperationState app={application} />{' '}
                            </a>
                            {appOperationState.syncResult && appOperationState.syncResult.revision && (
                                <div className='application-status-panel__item-value__revision show-for-large'>
                                    to <Revision repoUrl={source.repoURL} revision={appOperationState.syncResult.revision} />
                                </div>
                            )}
                        </div>

                        <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                            {appOperationState.phase} <Timestamp date={appOperationState.finishedAt || appOperationState.startedAt} />
                        </div>
                        {(appOperationState.syncResult && appOperationState.syncResult.revision && (
                            <RevisionMetadataPanel
                                appName={application.metadata.name}
                                appNamespace={application.metadata.namespace}
                                type={source.chart && 'helm'}
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
                input={application}
                load={async app => {
                    return await services.applications.getApplicationSyncWindowState(app.metadata.name, app.metadata.namespace);
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
