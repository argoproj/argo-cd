import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {DataLoader, InfoPopup} from '../../../shared/components';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as utils from '../utils';
import {ApplicationSyncWindowStatusIcon, ComparisonStatusIcon, getAppOperationState, HealthStatusIcon, OperationState, syncStatusMessage} from '../utils';
import {RevisionMetadataPanel} from './revision-metadata-panel';

require('./application-status-panel.scss');

interface Props {
    application: models.Application;
    showOperation?: () => any;
    showConditions?: () => any;
}

export const ApplicationStatusPanel = ({application, showOperation, showConditions}: Props) => {
    const today = new Date();
    const [title, setTitle] = React.useState<JSX.Element>();
    const [content, setContent] = React.useState<JSX.Element>();

    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    const cntByCategory = (application.status.conditions || []).reduce(
        (map, next) => map.set(utils.getConditionCategory(next), (map.get(utils.getConditionCategory(next)) || 0) + 1),
        new Map<string, number>()
    );
    const appOperationState = getAppOperationState(application);
    if (application.metadata.deletionTimestamp) {
        showOperation = null;
    }

    function viewApplicationStatusFull(infoTitle: JSX.Element, infoContent: JSX.Element) {
        setTitle(infoTitle);
        setContent(infoContent);
    }

    function closeApplicationStatusFull() {
        setTitle(null);
        setContent(null);
    }

    return (
        <div className='application-status-panel row'>
            {title && content && <InfoPopup title={title} content={content} onClose={closeApplicationStatusFull} />}
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
                        <RevisionMetadataPanel appName={application.metadata.name} type={application.spec.source.chart && 'helm'} revision={application.status.sync.revision} />
                    )}
                </div>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item columns small-4'>
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
                    <div className='third-column-truncate'>
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
                            />
                        )) || <div className='application-status-panel__item-name'>{appOperationState.message}</div>}
                        <button
                            className='view-application-status-full'
                            onClick={() =>
                                viewApplicationStatusFull(
                                    // title of info popup
                                    <div className={`info-popup-title application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                                        <a onClick={() => showOperation && showOperation()}>
                                            {utils.getOperationStateTitle(application)}
                                            <div className='info-popup-title__timestamp'>
                                                - {appOperationState.phase} <Timestamp date={appOperationState.finishedAt || appOperationState.startedAt} />
                                            </div>
                                        </a>
                                    </div>,
                                    // content of info-popup
                                    <div className='info-popup-content'>
                                        {appOperationState.syncResult && appOperationState.syncResult.revision && (
                                            <div className='info-popup-content__data'>
                                                To <Revision repoUrl={application.spec.source.repoURL} revision={appOperationState.syncResult.revision} />
                                            </div>
                                        )}
                                        {(appOperationState.syncResult && appOperationState.syncResult.revision && (
                                            <div className='info-popup-content__data'>
                                                <RevisionMetadataPanel
                                                    appName={application.metadata.name}
                                                    type={application.spec.source.chart && 'helm'}
                                                    revision={appOperationState.syncResult.revision}
                                                />
                                            </div>
                                        )) || <div className='info-popup-content__data'>{appOperationState.message}</div>}
                                        <br />
                                        <div>It has been {daysSinceLastSynchronized} days since last sync.</div>
                                    </div>
                                )
                            }>
                            Expand <i className='fas fa-angle-double-right' />
                        </button>
                    </div>
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
