import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import * as utils from '../utils';
import {ComparisonStatusIcon, HealthStatusIcon, syncStatusMessage} from '../utils';
import {RevisionMetadataPanel} from './revision-metadata-panel';

require('./application-status-panel.scss');

interface Props {
    application: models.Application;
    showOperation?: () => any;
    showConditions?: () => any;
}

export const ApplicationStatusPanel = ({application, showOperation, showConditions}: Props) => {
    const today = new Date();

    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    let appOperationState = application.status.operationState;
    if (application.metadata.deletionTimestamp) {
        appOperationState = {
            phase: models.OperationPhases.Running,
            startedAt: application.metadata.deletionTimestamp,
        } as models.OperationState;
        showOperation = null;
    } else if (application.operation) {
        appOperationState = {
            phase: models.OperationPhases.Running,
            startedAt: new Date().toISOString(),
            operation: {
                sync: {},
            } as models.Operation,
        } as models.OperationState;
    }

    const tooltip = (title: string) => (
        <Tooltip content={title}>
            <span style={{fontSize: 'smaller'}}> <i className='fa fa-question-circle help-tip'/></span>
        </Tooltip>
    );

    const conditionLink = (category: string) => {
        const conditions = (application.status.conditions || []).filter((c) => utils.getConditionCategory(c) === category);
        const icon = category === 'info' ? 'fa fa-check-circle' : category === 'warning' ?  'fa fa-exclamation-circle' : 'fa fa-times';
        switch (conditions.length) {
            case 0:
                return null;
            case 1:
                return <a className={category}><i className={icon}/> {conditions[0].message}</a>;
            default:
                return <a className={category}><i className={icon}/> {conditions[0].message} and {conditions.length - 1} other {category}s</a>;
        }
    };

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item columns small-3'>
                <div className='application-status-panel__item-value'>
                    <HealthStatusIcon state={application.status.health}/>&nbsp;
                    {application.status.health.status}
                    {tooltip('The health status of your app')}
                </div>
                <div className='application-status-panel__item-name'>{application.status.health.message}</div>
            </div>
            <div className='application-status-panel__item columns small-3' style={{position: 'relative'}}>
                <div className='application-status-panel__item-value'>
                    <ComparisonStatusIcon status={application.status.sync.status}/>&nbsp;
                    {application.status.sync.status}
                    {tooltip('Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.')}
                </div>
                <div className='application-status-panel__item-name'>{syncStatusMessage(application)}</div>
                <div className='application-status-panel__item-name'>
                    { application.status && application.status.sync && (
                        <RevisionMetadataPanel applicationName={application.metadata.name} revision={application.status.sync.revision}/>
                    )}
                </div>
            </div>
            {appOperationState && (
                <div className='application-status-panel__item columns small-3'>
                    <div
                        className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                        <a onClick={() => showOperation && showOperation()}>{utils.getOperationType(application)}
                            <utils.OperationPhaseIcon phase={appOperationState.phase}/>
                        </a>
                        {tooltip('Whether or not your last app sync was successful. It has been ' + daysSinceLastSynchronized +
                            ' days since last sync. Click for the status of that sync.')}
                    </div>
                    {appOperationState.syncResult && (
                        <div className='application-status-panel__item-name'>To <Revision
                            repoUrl={application.spec.source.repoURL} revision={appOperationState.syncResult.revision}/>
                        </div>
                    )}
                    <div className='application-status-panel__item-name'>
                        {appOperationState.phase} <Timestamp
                        date={appOperationState.finishedAt || appOperationState.startedAt}/>
                    </div>
                    {appOperationState.syncResult && (
                        <RevisionMetadataPanel applicationName={application.metadata.name}
                                               revision={appOperationState.syncResult.revision}/>
                    )}
                </div>
            )}
            {application.status.conditions && (
                <div className={`application-status-panel__item columns small-3`} onClick={() => showConditions && showConditions()}>
                    <div>{conditionLink( 'error')}</div>
                    <div>{conditionLink('warning')}</div>
                    <div>{conditionLink('info')}</div>
                </div>
            )}
        </div>
    );
};
