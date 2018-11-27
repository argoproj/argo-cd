import { Duration, NotificationType, Ticker } from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import { ErrorNotification } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import * as utils from '../utils';

interface Props { application: models.Application; operationState: models.OperationState; }

export const ApplicationOperationState: React.StatelessComponent<Props> = ({application, operationState}, ctx: AppContext) => {

    const operationAttributes = [
        {title: 'OPERATION', value: utils.getOperationType(application)},
        {title: 'PHASE', value: operationState.phase},
        ...(operationState.message ? [{title: 'MESSAGE', value: operationState.message}] : []),
        {title: 'STARTED AT', value: operationState.startedAt},
        {title: 'DURATION', value: (
            <Ticker>
                {(time) => <Duration durationMs={(operationState.finishedAt && moment(operationState.finishedAt) || time).diff(moment(operationState.startedAt)) / 1000}/>}
            </Ticker>
        )},
    ];

    if (operationState.finishedAt) {
        operationAttributes.push({ title: 'FINISHED AT', value: operationState.finishedAt});
    } else if (operationState.phase !== 'Terminating') {
        operationAttributes.push({
            title: '',
            value: (
                <button className='argo-button argo-button--base' onClick={async () => {
                    const confirmed = await ctx.apis.popup.confirm('Terminate operation', 'Are you sure you want to terminate operation?');
                    if (confirmed) {
                        try {
                            services.applications.terminateOperation(application.metadata.name);
                        } catch (e) {
                            ctx.apis.notifications.show({
                                content: <ErrorNotification title='Unable to terminate operation' e={e}/>,
                                type: NotificationType.Error,
                            });
                        }
                    }
                }}>Terminate</button>
            ),
        });
    }

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

ApplicationOperationState.contextTypes = {
    apis: PropTypes.object,
};
