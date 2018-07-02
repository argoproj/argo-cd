import * as React from 'react';

import * as models from '../../../shared/models';
import * as utils from '../utils';

require('./application-status-panel.scss');

export const ApplicationStatusPanel = ({application, onClick}: { application: models.Application, onClick?: () => any}) => {
    const today = new Date();
    const creationDate = new Date(application.metadata.creationTimestamp);
    const conditionsCount = application.status.conditions && application.status.conditions.length || 0;

    const daysActive = Math.round(Math.abs((today.getTime() - creationDate.getTime()) / (24 * 60 * 60 * 1000)));
    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
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
                <div className='application-status-panel__item-value'>
                    <a onClick={() => onClick && onClick()}>{utils.getOperationType(application.status.operationState)}</a>
                </div>
                <div className='application-status-panel__item-name'>
                    {application.status.operationState.phase} at {application.status.operationState.finishedAt || application.status.operationState.startedAt}
                </div>
            </div>
            )}
            <div className='application-status-panel__item columns small-3'>
                <div className='application-status-panel__item-value'>
                    <span className={conditionsCount && "warning"}>{conditionsCount}</span>
                </div>
                <div className='application-status-panel__item-name'>
                    <span className={conditionsCount && "warning"}>Warnings</span>
                </div>
            </div>
        </div>
    );
};
