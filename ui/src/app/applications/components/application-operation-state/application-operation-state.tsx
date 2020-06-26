import {Duration, NotificationType, Ticker} from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import {ErrorNotification, Revision, Timestamp} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as utils from '../utils';

require('./application-operation-state.scss');

interface Props {
    application: models.Application;
    operationState: models.OperationState;
}

export const ApplicationOperationState: React.StatelessComponent<Props> = ({application, operationState}, ctx: AppContext) => {
    const operationAttributes = [
        {title: 'OPERATION', value: utils.getOperationType(application)},
        {title: 'PHASE', value: operationState.phase},
        ...(operationState.message ? [{title: 'MESSAGE', value: operationState.message}] : []),
        {title: 'STARTED AT', value: <Timestamp date={operationState.startedAt} />},
        {
            title: 'DURATION',
            value: (
                <Ticker>
                    {time => <Duration durationMs={((operationState.finishedAt && moment(operationState.finishedAt)) || time).diff(moment(operationState.startedAt)) / 1000} />}
                </Ticker>
            )
        }
    ];

    if (operationState.finishedAt && operationState.phase !== 'Running') {
        operationAttributes.push({title: 'FINISHED AT', value: <Timestamp date={operationState.finishedAt} />});
    } else if (operationState.phase !== 'Terminating') {
        operationAttributes.push({
            title: '',
            value: (
                <button
                    className='argo-button argo-button--base'
                    onClick={async () => {
                        const confirmed = await ctx.apis.popup.confirm('Terminate operation', 'Are you sure you want to terminate operation?');
                        if (confirmed) {
                            try {
                                await services.applications.terminateOperation(application.metadata.name);
                            } catch (e) {
                                ctx.apis.notifications.show({
                                    content: <ErrorNotification title='Unable to terminate operation' e={e} />,
                                    type: NotificationType.Error
                                });
                            }
                        }
                    }}>
                    Terminate
                </button>
            )
        });
    }
    if (operationState.syncResult) {
        operationAttributes.push({title: 'REVISION', value: <Revision repoUrl={application.spec.source.repoURL} revision={operationState.syncResult.revision} />});
    }
    let initiator = '';
    if (operationState.operation.initiatedBy) {
        if (operationState.operation.initiatedBy.automated) {
            initiator = 'automated sync policy';
        } else {
            initiator = operationState.operation.initiatedBy.username;
        }
    }
    operationAttributes.push({title: 'INITIATED BY', value: initiator || 'Unknown'});

    const resultAttributes: {title: string; value: string}[] = [];
    const syncResult = operationState.syncResult;
    if (operationState.finishedAt) {
        if (syncResult) {
            (syncResult.resources || []).forEach(res => {
                resultAttributes.push({
                    title: `${res.namespace}/${res.kind}:${res.name}`,
                    value: res.message
                });
            });
        }
    }

    return (
        <div>
            <div className='white-box'>
                <div className='white-box__details'>
                    {operationAttributes.map(attr => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>{attr.title}</div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
            {syncResult && syncResult.resources && syncResult.resources.length > 0 && (
                <React.Fragment>
                    <h4>Result:</h4>
                    <div className='argo-table-list'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns large-1 show-for-large application-operation-state__icons_container_padding'>KIND</div>
                                <div className='columns large-2 show-for-large'>NAMESPACE</div>
                                <div className='columns large-2 small-2'>NAME</div>
                                <div className='columns large-1 small-2'>STATUS</div>
                                <div className='columns large-1 show-for-large'>HOOK</div>
                                <div className='columns large-4 small-8'>MESSAGE</div>
                            </div>
                        </div>
                        {syncResult.resources.map((resource, i) => (
                            <div className='argo-table-list__row' key={i}>
                                <div className='row'>
                                    <div className='columns large-1 show-for-large application-operation-state__icons_container_padding'>
                                        <div className='application-operation-state__icons_container'>
                                            {resource.hookType && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                                        </div>
                                        <span title={getKind(resource)}>{getKind(resource)}</span>
                                    </div>
                                    <div className='columns large-2 show-for-large' title={resource.namespace}>
                                        {resource.namespace}
                                    </div>
                                    <div className='columns large-2 small-2' title={resource.name}>
                                        {resource.name}
                                    </div>
                                    <div className='columns large-1 small-2' title={getStatus(resource)}>
                                        <utils.ResourceResultIcon resource={resource} /> {getStatus(resource)}
                                    </div>
                                    <div className='columns large-1 show-for-large' title={resource.hookType}>
                                        {resource.hookType}
                                    </div>
                                    <div className='columns large-4 small-8' title={resource.message}>
                                        <div className='application-operation-state__message'>{resource.message}</div>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                </React.Fragment>
            )}
        </div>
    );
};

const getKind = (resource: models.ResourceResult): string => {
    return (resource.group ? `${resource.group}/${resource.version}` : resource.version) + `/${resource.kind}`;
};

const getStatus = (resource: models.ResourceResult): string => {
    return resource.hookType ? resource.hookPhase : resource.status;
};

ApplicationOperationState.contextTypes = {
    apis: PropTypes.object
};
