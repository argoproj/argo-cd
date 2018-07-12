import { Duration } from 'argo-ui';
import * as moment from 'moment';
import * as React from 'react';

import { Ticker } from '../../../shared/components';
import * as models from '../../../shared/models';
import * as utils from '../utils';

export const ApplicationOperationState = ({operationState}: { operationState: models.OperationState }) => {

    const operationAttributes = [
        {title: 'OPERATION', value: utils.getOperationType(operationState)},
        {title: 'PHASE', value: operationState.phase},
        ...(operationState.message ? [{title: 'MESSAGE', value: operationState.message}] : []),
        {title: 'STARTED AT', value: operationState.startedAt},
        {title: 'DURATION', value: (
            <Ticker>
                {(time) => <Duration durationMs={(operationState.finishedAt && moment(operationState.finishedAt) || time).diff(moment(operationState.startedAt)) / 1000}/>}
            </Ticker>
        )},
        ...(operationState.finishedAt ? [{title: 'FINISHED AT', value: operationState.finishedAt}] : []),
    ];

    const resultAttributes: {title: string, value: string}[] = [];
    const syncResult = operationState.syncResult || operationState.rollbackResult;
    if (operationState.finishedAt) {
        if (syncResult) {
            (syncResult.resources || []).forEach((res) => {
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
            {syncResult && syncResult.hooks && syncResult.hooks.length > 0 && (
                <React.Fragment>
                    <h4>Hooks:</h4>
                    <div className='argo-table-list'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns small-2'>
                                    KIND
                                </div>
                                <div className='columns small-2'>
                                    NAME
                                </div>
                                <div className='columns small-2'>
                                    STATUS
                                </div>
                                <div className='columns small-2'>
                                    TYPE
                                </div>
                                <div className='columns small-4'>
                                    MESSAGE
                                </div>
                            </div>
                        </div>
                    {syncResult.hooks.map((hook, i) => (
                        <div className='argo-table-list__row' key={i}>
                            <div className='row'>
                                <div className='columns small-2'>
                                    {hook.kind}
                                </div>
                                <div className='columns small-2'>
                                    {hook.name}
                                </div>
                                <div className='columns small-2'>
                                    {hook.status}
                                </div>
                                <div className='columns small-2'>
                                    {hook.type}
                                </div>
                                <div className='columns small-4'>
                                    {hook.message}
                                </div>
                            </div>
                        </div>
                    ))}
                    </div>
                </React.Fragment>
            )}
            {resultAttributes.length > 0 && (
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
