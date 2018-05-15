import * as React from 'react';

import * as models from '../../../shared/models';

require('./application-status-panel.scss');

function getOperationType(state: models.OperationState) {
    if (state.operation.sync) {
        return 'synchronization';
    } else if (state.operation.rollback) {
        return 'rollback';
    }
    return 'unknown operation';
}

export const ApplicationStatusPanel = ({application}: { application: models.Application }) => {
    const today = new Date();
    const creationDate = new Date(application.metadata.creationTimestamp);

    const daysActive = Math.round(Math.abs((today.getTime() - creationDate.getTime()) / (24 * 60 * 60 * 1000)));
    let daysSinceLastSynchronized = 0;
    const history = application.status.recentDeployments || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item columns small-3'>
                <div className='application-status-panel__item-value'>{daysActive}</div>
                <div className='application-status-panel__item-name'>Days active</div>
            </div>
            <div className='application-status-panel__item columns small-3'>
                <div className='application-status-panel__item-value'>{daysSinceLastSynchronized}</div>
                <div className='application-status-panel__item-name'>Days since last synchronized</div>
            </div>
            {application.status.operationState && (
            <div className='application-status-panel__item columns small-3'>
                <div className='application-status-panel__item-value'>{getOperationType(application.status.operationState)}</div>
                <div className='application-status-panel__item-name'>
                    {application.status.operationState.phase} at {application.status.operationState.finishedAt || application.status.operationState.startedAt}
                </div>
            </div>
            )}
        </div>
    );
};
