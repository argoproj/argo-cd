import { Duration } from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';

import * as models from '../../../shared/models';
import * as utils from '../utils';

export const ApplicationOperationState = ({operationState}: { operationState: models.OperationState }) => {
    const durationMs = operationState.finishedAt ?
        moment(operationState.finishedAt).diff(moment(operationState.startedAt)) / 1000 :
        null;

    const operationAttributes = [
        {title: 'OPERATION', value: utils.getOperationType(operationState)},
        {title: 'PHASE', value: operationState.phase},
        {title: 'STARTED AT', value: operationState.startedAt},
        {title: 'FINISHED AT', value: operationState.finishedAt},
        ...(durationMs ? [{title: 'DURATION', value: <Duration durationMs={durationMs}/>}] : []),
    ];

    const resultAttributes: {title: string, value: string}[] = [];
    if (operationState.finishedAt) {
        if (operationState.message) {
            resultAttributes.push({title: 'MESSAGE', value: operationState.message});
        }
        const syncResult = operationState.syncResult || operationState.rollbackResult;
        if (syncResult) {
            syncResult.resources.forEach((res) => {
                resultAttributes.push({
                    title: `${res.namespace}/${res.kind}:${res.name}`,
                    value: res.message,
                });
            });
        }
    }
    return (
        <div>
            <div className='white-box'>
                <div className='white-box__details'>
                    {operationAttributes.map((attr) => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>
                                {attr.title}
                            </div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
            { resultAttributes.length > 0 && (
                <h4>Result details:</h4>
            )}
            { resultAttributes.length > 0 && (
                <div className='white-box'>
                    <div className='white-box__details'>
                        {resultAttributes.map((attr) => (
                            <div className='row white-box__details-row' key={attr.title}>
                                <div className='columns small-3'>
                                    {attr.title}
                                </div>
                                <div className='columns small-9'>{attr.value}</div>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
};
