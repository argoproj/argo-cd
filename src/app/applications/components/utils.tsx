import * as React from 'react';

import { Checkbox, NotificationType } from 'argo-ui';
import { ARGO_FAILED_COLOR, ARGO_RUNNING_COLOR, ARGO_SUCCESS_COLOR, ErrorNotification } from '../../shared/components';
import { AppContext } from '../../shared/context';
import * as appModels from '../../shared/models';
import { services } from '../../shared/services';

export async function deleteApplication(appName: string, context: AppContext, success: () => void) {
    let cascade = false;
    const confirmationForm = class extends React.Component<{}, { cascade: boolean } > {
        constructor(props: any) {
            super(props);
            this.state = {cascade: false};
        }
        public render() {
            return (
                <div>
                    <p>Are you sure you want to delete the application "{appName}"?</p>
                    <p><Checkbox checked={this.state.cascade} onChange={(val) => this.setState({ cascade: val })} /> Cascade</p>
                </div>
            );
        }
        public componentWillUnmount() {
            cascade = this.state.cascade;
        }
    };
    const confirmed = await context.apis.popup.confirm('Delete application', confirmationForm);
    if (confirmed) {
        try {
            await services.applications.delete(appName, cascade);
            success();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to delete application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }
}

export const ComparisonStatusIcon = ({status}: { status: appModels.ComparisonStatus }) => {
    let className = '';
    let color = '';

    switch (status) {
        case appModels.ComparisonStatuses.Synced:
            className = 'fa fa-check-circle';
            color = ARGO_SUCCESS_COLOR;
            break;
        case appModels.ComparisonStatuses.OutOfSync:
            className = 'fa fa-times';
            color = ARGO_FAILED_COLOR;
            break;
        case appModels.ComparisonStatuses.Error:
            className = 'fa fa-exclamation-circle';
            color = ARGO_FAILED_COLOR;
            break;
        case appModels.ComparisonStatuses.Unknown:
            className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
            color = ARGO_RUNNING_COLOR;
            break;
    }
    return <i title={status} className={className} style={{ color }} />;
};

export const HealthStatusIcon = ({state}: { state: appModels.HealthStatus }) => {
    let color = '';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            color = ARGO_SUCCESS_COLOR;
            break;
        case appModels.HealthStatuses.Degraded:
            color = ARGO_FAILED_COLOR;
            break;
        case appModels.HealthStatuses.Progressing:
            color = ARGO_RUNNING_COLOR;
            break;
        case appModels.HealthStatuses.Unknown:
            color = ARGO_RUNNING_COLOR;
            break;
    }
    let title: string = state.status;
    if (state.statusDetails) {
        title = `${state.status}: ${state.statusDetails};`;
    }
    return <i title={title} className='fa fa-heartbeat' style={{ color }} />;
};

export function getStateAndNode(resource: appModels.ResourceNode | appModels.ResourceState) {
    let resourceNode: appModels.ResourceNode;
    let resourceState = resource as appModels.ResourceState;
    if (resourceState.liveState || resourceState.targetState) {
        resourceNode = { state: resourceState.liveState || resourceState.targetState, children: resourceState.childLiveResources };
    } else {
        resourceState = null;
        resourceNode = resource as appModels.ResourceNode;
    }
    return {resourceState, resourceNode};
}

export function getOperationType(state: appModels.OperationState) {
    if (state.operation.sync) {
        return 'synchronization';
    } else if (state.operation.rollback) {
        return 'rollback';
    }
    return 'unknown operation';
}

export function getPodStateReason(pod: appModels.State): { message: string; reason: string } {
    let reason = pod.status.phase;
    let message = '';
    if (pod.status.reason) {
        reason = pod.status.reason;
    }

    let initializing = false;
    for (const container of (pod.status.initContainerStatuses || []).slice().reverse()) {
        if (container.state.terminated && container.state.terminated.exitCode === 0) {
            continue;
        }

        if (container.state.terminated) {
            if (container.state.terminated.reason) {
                reason = `Init:ExitCode:${container.state.terminated.exitCode}`;
            } else {
                reason = `Init:${container.state.terminated.reason}`;
                message = container.state.terminated.message;
            }
        } else if (container.state.waiting && container.state.waiting.reason && container.state.waiting.reason !== 'PodInitializing') {
            reason = `Init:${container.state.waiting.reason}`;
            message = `Init:${container.state.waiting.message}`;
        } else {
            reason = `Init: ${(pod.spec.initContainers || []).length})`;
        }
        initializing = true;
        break;
    }

    if (!initializing) {
        let hasRunning = false;
        for (const container of (pod.status.containerStatuses || [])) {
            if (container.state.waiting && container.state.waiting.reason) {
                reason = container.state.waiting.reason;
                message = container.state.waiting.message;
            } else if (container.state.terminated && container.state.terminated.reason ) {
                reason = container.state.terminated.reason;
                message = container.state.terminated.message;
            } else if (container.state.terminated && container.state.terminated.reason) {
                if (container.state.terminated.signal !== 0) {
                    reason = `Signal:${container.state.terminated.signal}`;
                    message = '';
                } else {
                    reason = `ExitCode:${container.state.terminated.exitCode}`;
                    message = '';
                }
            } else if (container.ready && container.state.running) {
                hasRunning = true;
            }
        }

        // change pod status back to "Running" if there is at least one container still reporting as "Running" status
        if (reason === 'Completed' && hasRunning) {
            reason = 'Running';
            message = '';
        }
    }

    if ((pod as any).deletionTimestamp && pod.status.reason === 'NodeLost') {
        reason = 'Unknown';
        message = '';
    } else if ((pod as any).deletionTimestamp) {
        reason = 'Terminating';
        message = '';
    }

    return {reason, message};
}

export function getConditionCategory(condition: appModels.ApplicationCondition): 'error' | 'warning' | 'info' {
    if (condition.type.endsWith('Error')) {
        return 'error';
    } else if (condition.type.endsWith('Warning')) {
        return 'warning';
    } else {
        return 'info';
    }
}
