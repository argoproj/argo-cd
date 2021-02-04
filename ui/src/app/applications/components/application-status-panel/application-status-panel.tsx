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
    if (application.metadata.deletionTimestamp) {
        showOperation = null;
    }

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item columns small-2'>
                <div className='application-status-panel__item-value'>
                    <HealthStatusIcon state={application.status.health} />
                    &nbsp;
                    {application.status.health.status}
                    <HelpIcon title='The health status of your app' />
                </div>
                <div className='application-status-panel__item-name'>{application.status.health.message}</div>
            </div>
            <div className='application-status-panel__item columns small-2' style={{position: 'relative'}}>
                <div className='application-status-panel__item-value'>
                    <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                    <HelpIcon title='Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.' />
                </div>
                <div className='application-status-panel__item-name'>{syncStatusMessage(application)}</div>
                <div className='application-status-panel__item-name'>
                    {application.status && application.status.sync && application.status.sync.revision && (
                        <RevisionMetadataPanel
                            appName={application.metadata.name}
                            type={application.spec.source.chart && 'helm'}
                            revision={application.status.sync.revision}
                            showInfo={showMetadataInfo && (info => showMetadataInfo(application.status.sync.revision, info))}
                        />
                    )}
                </div>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item columns small-4 '>
                    <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                        <a onClick={() => showOperation && showOperation()}>
                            <OperationState app={application} />
                            <HelpIcon
                                title={
                                    'Whether or not your last app sync was successful. It has been ' +
                                    daysSinceLastSynchronized +
                                    ' days since last sync. Click for the status of that sync.'
                                }
                            />
                        </a>
                    </div>
                    {appOperationState.syncResult && appOperationState.syncResult.revision && (
                        <div className='application-status-panel__item-name'>
                            To <Revision repoUrl={application.spec.source.repoURL} revision={appOperationState.syncResult.revision} />
                        </div>
                    )}
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
                <div className={`application-status-panel__item columns small-2`}>
                    <div className='application-status-panel__item-value' onClick={() => showConditions && showConditions()}>
                        {cntByCategory.get('info') && <a className='info'>{cntByCategory.get('info')} Info</a>}
                        {cntByCategory.get('warning') && <a className='warning'>{cntByCategory.get('warning')} Warnings</a>}
                        {cntByCategory.get('error') && <a className='error'>{cntByCategory.get('error')} Errors</a>}
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
                        <div className='application-status-panel__item columns small-2' style={{position: 'relative'}}>
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
