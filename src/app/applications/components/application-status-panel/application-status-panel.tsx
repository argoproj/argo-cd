import { DropDownMenu } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import * as models from '../../../shared/models';
import * as utils from '../utils';
import { ComparisonStatusIcon, HealthStatusIcon, syncStatusMessage } from '../utils';

require('./application-status-panel.scss');

interface Props {
    application: models.Application;
    refresh: (force: boolean) => any;
    showOperation?: () => any;
    showConditions?: () => any;
}

export const ApplicationStatusPanel = ({application, showOperation, showConditions, refresh}: Props) => {
    const refreshing = application.metadata.annotations && application.metadata.annotations[models.AnnotationRefreshKey];
    const today = new Date();
    const creationDate = new Date(application.metadata.creationTimestamp);

    const daysActive = Math.round(Math.abs((today.getTime() - creationDate.getTime()) / (24 * 60 * 60 * 1000)));
    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    const cntByCategory = (application.status.conditions || []).reduce(
        (map, next) => map.set(utils.getConditionCategory(next), (map.get(utils.getConditionCategory(next)) || 0) + 1),
        new Map<string, number>());
    let appOperationState = application.status.operationState;
    if (application.metadata.deletionTimestamp) {
        appOperationState = {
            phase:  models.OperationPhases.Running,
            startedAt: application.metadata.deletionTimestamp,
        } as models.OperationState;
        showOperation = null;
    } else if (application.operation && application.status.operationState === undefined) {
        appOperationState = {
            phase:  models.OperationPhases.Running,
            startedAt: new Date().toISOString(),
            operation: {
                sync: {},
            } as models.Operation,
        } as models.OperationState;
    }
    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item columns small-2'>
                <div className='application-status-panel__item-value'>{daysActive}</div>
                <div className='application-status-panel__item-name'>Days active</div>
            </div>
            <div className='application-status-panel__item columns small-2'>
                <div className='application-status-panel__item-value'>{daysSinceLastSynchronized}</div>
                <div className='application-status-panel__item-name'>Days since last synchronized</div>
            </div>
            <div className='application-status-panel__item columns small-2' style={{position: 'relative'}}>
                <div className='application-status-panel__item-value'><ComparisonStatusIcon status={application.status.sync.status}/> {application.status.sync.status}</div>
                <div className='application-status-panel__item-name'>{syncStatusMessage(application)}</div>
                <div className={classNames('application-status-panel__refresh', { 'application-status-panel__refresh--disabled': !!refreshing })}>
                    <i className={classNames('fa fa-refresh', { 'status-icon--spin': !!refreshing })}
                        onClick={() => !refreshing && refresh(false)}/> <DropDownMenu items={[{
                        title: 'Hard Refresh', action: () => !refreshing && refresh(true),
                    }]} anchor={() => <i className='fa fa-caret-down'/>} />
                </div>
            </div>
            <div className='application-status-panel__item columns small-2'>
                <div className='application-status-panel__item-value'><HealthStatusIcon state={application.status.health}/> {application.status.health.status}</div>
                <div className='application-status-panel__item-name'>{application.status.health.message}</div>
            </div>
            {appOperationState && (
            <div className='application-status-panel__item columns small-2'>
                <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                    <a onClick={() => showOperation && showOperation()}>{utils.getOperationType(application)} <utils.OperationPhaseIcon
                        phase={appOperationState.phase}/></a>
                </div>
                <div className='application-status-panel__item-name'>
                    {appOperationState.phase} at {appOperationState.finishedAt || appOperationState.startedAt}
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
        </div>
    );
};
