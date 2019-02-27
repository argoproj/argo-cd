import * as React from 'react';

import { Checkbox, NotificationType } from 'argo-ui';
import { ARGO_FAILED_COLOR, ARGO_RUNNING_COLOR, ARGO_SUCCESS_COLOR, ErrorNotification, ARGO_GRAY4_COLOR } from '../../shared/components';
import { AppContext } from '../../shared/context';
import * as appModels from '../../shared/models';
import { services } from '../../shared/services';

export const ICON_CLASS_BY_KIND = {
    application: 'argo-icon-application',
    deployment: 'argo-icon-deployment',
    pod: 'argo-icon-docker',
    service: 'argo-icon-hosts',
} as any;

export interface ResourceTreeNode extends appModels.ResourceNode {
    status?: appModels.SyncStatusCode;
    health?: appModels.HealthStatus;
    hook?: boolean;
    root?: ResourceTreeNode;
}

export interface NodeId {
    kind: string;
    namespace: string;
    name: string;
    group: string;
}

export function nodeKey(node: NodeId) {
    return [node.group === 'extensions' ? 'apps' : node.group, node.kind, node.namespace, node.name].join(':');
}

export function isSameNode(first: NodeId, second: NodeId) {
    return nodeKey(first) === nodeKey(second);
}

export function getParamsWithOverridesInfo(params: appModels.ComponentParameter[], overrides: appModels.ComponentParameter[]) {
    const componentParams = new Map<string, (appModels.ComponentParameter & {original: string})[]>();
    (params || []).map((param) => ({component: '', ...param})).map((param) => {
        const override = (overrides || [])
            .map((item) => ({component: '', ...item}))
            .find((item) => item.component === param.component && item.name === param.name);
        const res = {...param, original: ''};
        if (override) {
            res.original = res.value;
            res.value = override.value;
        }
        return res;
    }).forEach((param) => {
        const items = componentParams.get(param.component) || [];
        items.push(param);
        componentParams.set(param.component, items);
    });
    return componentParams;
}

export async function syncApplication(appName: string, revision: string, prune: boolean, dryRun: boolean, resources: appModels.SyncOperationResource[], context: AppContext) {
    try {
        await services.applications.sync(appName, revision, prune, dryRun, resources);
        return true;
    } catch (e) {
        context.apis.notifications.show({
            content: <ErrorNotification title='Unable to deploy revision' e={e}/>,
            type: NotificationType.Error,
        });
    }
    return false;
}

export async function deleteApplication(appName: string, context: AppContext): Promise<boolean> {
    let cascade = false;
    const confirmationForm = class extends React.Component<{}, { cascade: boolean } > {
        constructor(props: any) {
            super(props);
            this.state = {cascade: true};
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
            return true;
        } catch (e) {
            context.apis.notifications.show({
                content: <ErrorNotification title='Unable to delete application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }
    return false;
}

export const OperationPhaseIcon = ({phase}: { phase: appModels.OperationPhase }) => {
    let className = '';
    let color = '';

    switch (phase) {
        case appModels.OperationPhases.Succeeded:
            className = 'fa fa-check-circle';
            color = ARGO_SUCCESS_COLOR;
            break;
        case appModels.OperationPhases.Error:
        case appModels.OperationPhases.Failed:
            className = 'fa fa-times';
            color = ARGO_FAILED_COLOR;
            break;
        default:
            className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
            color = ARGO_RUNNING_COLOR;
            break;
    }
    return <i title={phase} className={className} style={{ color }} />;
};

export const ComparisonStatusIcon = ({status}: { status: appModels.SyncStatusCode }) => {
    let className = '';
    let color = '';

    switch (status) {
        case appModels.SyncStatuses.Synced:
            className = 'fa fa-check-circle';
            color = ARGO_SUCCESS_COLOR;
            break;
        case appModels.SyncStatuses.OutOfSync:
            className = 'fa fa-times';
            color = ARGO_FAILED_COLOR;
            break;
        case appModels.SyncStatuses.Unknown:
            className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
            color = ARGO_RUNNING_COLOR;
            break;
    }
    return <i title={status} className={className} style={{ color }} />;
};

export function syncStatusMessage(app: appModels.Application) {
    let message = '';
    let rev = app.spec.source.targetRevision || 'HEAD';
    if (app.status.sync.revision.length >= 7 && !app.status.sync.revision.startsWith(app.spec.source.targetRevision)) {
        rev += ' (' + app.status.sync.revision.substr(0, 7) + ')';
    }
    switch (app.status.sync.status) {
        case appModels.SyncStatuses.Synced:
            message += ' to ' + rev;
            break;
        case appModels.SyncStatuses.OutOfSync:
            message += ' from ' + rev;
            break;
        case appModels.SyncStatuses.Unknown:
            break;
    }
    return message;
}

export const HealthStatusIcon = ({state}: { state: appModels.HealthStatus }) => {
    let color = '';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            color = ARGO_SUCCESS_COLOR;
            break;
        case appModels.HealthStatuses.Suspended:
            color = ARGO_GRAY4_COLOR;
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
    if (state.message) {
        title = `${state.status}: ${state.message};`;
    }
    return <i title={title} className='fa fa-heartbeat' style={{ color }} />;
};

export const ResourceResultIcon = ({resource}: { resource: appModels.ResourceResult }) => {
    let color = '';

    if (resource.status) {
        switch (resource.status) {
            case appModels.ResultCodes.Synced:
                color = ARGO_SUCCESS_COLOR;
                break;
            case appModels.ResultCodes.Pruned:
                color = ARGO_SUCCESS_COLOR;
                break;
            case appModels.ResultCodes.SyncFailed:
                color = ARGO_FAILED_COLOR;
                break;
            case appModels.ResultCodes.PruneSkipped:
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.status}: ${resource.message};`;
        }
        return <i title={title} className='fa fa-heartbeat' style={{ color }} />;
    }
    if (resource.hookPhase) {
        let className = '';
        switch (resource.hookPhase) {
            case appModels.OperationPhases.Running:
                color = ARGO_RUNNING_COLOR;
                className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
                break;
            case appModels.OperationPhases.Failed:
                color = ARGO_FAILED_COLOR;
                className = 'fa fa-heartbeat';
                break;
            case appModels.OperationPhases.Error:
                color = ARGO_FAILED_COLOR;
                className = 'fa fa-heartbeat';
                break;
            case appModels.OperationPhases.Succeeded:
                color = ARGO_SUCCESS_COLOR;
                className = 'fa fa-heartbeat';
                break;
            case appModels.OperationPhases.Terminating:
                color = ARGO_RUNNING_COLOR;
                className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.hookPhase}: ${resource.message};`;
        }
        return <i title={title} className={className} style={{ color }} />;
    }
    return null;
};

export function getOperationType(application: appModels.Application) {
    if (application.metadata.deletionTimestamp) {
        return 'deletion';
    }
    const operation = application.operation || application.status.operationState && application.status.operationState.operation;
    if (operation && operation.sync) {
        return 'synchronization';
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

    if ((pod as any).metadata.deletionTimestamp && pod.status.reason === 'NodeLost') {
        reason = 'Unknown';
        message = '';
    } else if ((pod as any).metadata.deletionTimestamp) {
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

export function isAppNode(node: ResourceTreeNode) {
    return node.kind === 'Application' && node.group === 'argoproj.io';
}
