import * as React from 'react';

import {Checkbox, NotificationsApi, NotificationType} from 'argo-ui';
import {COLORS, ErrorNotification} from '../../shared/components';
import {Revision} from '../../shared/components/revision';
import {ContextApis} from '../../shared/context';
import * as appModels from '../../shared/models';
import {services} from '../../shared/services';

export const ICON_CLASS_BY_KIND = {
    application: 'argo-icon-application',
    deployment: 'argo-icon-deployment',
    pod: 'argo-icon-docker',
    service: 'argo-icon-hosts',
} as any;

export interface NodeId {
    kind: string;
    namespace: string;
    name: string;
    group: string;
}

export function nodeKey(node: NodeId) {
    return [node.group, node.kind, node.namespace, node.name].join('/');
}

export function isSameNode(first: NodeId, second: NodeId) {
    return nodeKey(first) === nodeKey(second);
}

export async function deleteApplication(appName: string, apis: ContextApis): Promise<boolean> {
    let cascade = false;
    const confirmationForm = class extends React.Component<{}, { cascade: boolean }> {
        constructor(props: any) {
            super(props);
            this.state = {cascade: true};
        }

        public render() {
            return (
                <div>
                    <p>Are you sure you want to delete the application "{appName}"?</p>
                    <p><Checkbox checked={this.state.cascade}
                                 onChange={(val) => this.setState({cascade: val})}/> Cascade</p>
                </div>
            );
        }

        public componentWillUnmount() {
            cascade = this.state.cascade;
        }
    };
    const confirmed = await apis.popup.confirm('Delete application', confirmationForm);
    if (confirmed) {
        try {
            await services.applications.delete(appName, cascade);
            return true;
        } catch (e) {
            apis.notifications.show({
                content: <ErrorNotification title='Unable to delete application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }
    return false;
}

export async function createApplication(app: appModels.Application, notifications: NotificationsApi): Promise<boolean> {
    try {
        await services.applications.create(app);
        return true;
    } catch (e) {
        notifications.show({
            content: <ErrorNotification title='Unable to create application' e={e}/>,
            type: NotificationType.Error,
        });
    }
    return false;
}

export const OperationPhaseIcon = ({phase}: { phase: appModels.OperationPhase }) => {
    let className = '';
    let color = '';

    switch (phase) {
        case appModels.OperationPhases.Succeeded:
            className = 'fa fa-check-circle';
            color = COLORS.operation.success;
            break;
        case appModels.OperationPhases.Error:
            className = 'fa fa-times';
            color = COLORS.operation.error;
            break;
        case appModels.OperationPhases.Failed:
            className = 'fa fa-times';
            color = COLORS.operation.failed;
            break;
        default:
            className = 'fa fa-circle-notch fa-spin';
            color = COLORS.operation.running;
            break;
    }
    return <i title={phase} className={className} style={{color}}/>;
};

export const ComparisonStatusIcon = ({status, resource, label}: { status: appModels.SyncStatusCode, resource?: { requiresPruning?: boolean }, label?: boolean }) => {
    let className = 'fa fa-question-circle';
    let color = COLORS.sync.unknown;
    let title: string = status;

    switch (status) {
        case appModels.SyncStatuses.Synced:
            className = 'fa fa-check-circle';
            color = COLORS.sync.synced;
            break;
        case appModels.SyncStatuses.OutOfSync:
            const requiresPruning = resource && resource.requiresPruning;
            className = requiresPruning ? 'fa fa-times-circle' : 'fa fa-times';
            if (requiresPruning) {
                title = `${title} (requires pruning)`;
            }
            color = COLORS.sync.out_of_sync;
            break;
        case appModels.SyncStatuses.Unknown:
            className = 'fa fa-circle-notch fa-spin';
            break;
    }
    return <React.Fragment><i title={title} className={className} style={{color}}/> {label && title}</React.Fragment>;
};

export function syncStatusMessage(app: appModels.Application) {
    let rev = app.spec.source.targetRevision || 'HEAD';
    if (app.status.sync.revision && (app.status.sync.revision.length >= 7 && !app.status.sync.revision.startsWith(app.spec.source.targetRevision))) {
        rev += ' (' + app.status.sync.revision.substr(0, 7) + ')';
    }
    switch (app.status.sync.status) {
        case appModels.SyncStatuses.Synced:
            return (
                <span>To <Revision repoUrl={app.spec.source.repoURL}
                                   revision={app.spec.source.targetRevision || 'HEAD'}>{rev}</Revision> </span>
            );
        case appModels.SyncStatuses.OutOfSync:
            return (
                <span>From <Revision repoUrl={app.spec.source.repoURL}
                                     revision={app.spec.source.targetRevision || 'HEAD'}>{rev}</Revision> </span>
            );
        default:
            return <span>{rev}</span>;
    }
}

export const HealthStatusIcon = ({state}: { state: appModels.HealthStatus }) => {
    let color = COLORS.health.unknown;
    let icon = 'fa-question-circle';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            color = COLORS.health.healthy;
            icon = 'fa-heartbeat';
            break;
        case appModels.HealthStatuses.Suspended:
            color = COLORS.health.suspended;
            icon = 'fa-heartbeat';
            break;
        case appModels.HealthStatuses.Degraded:
            color = COLORS.health.degraded;
            icon = 'fa-heart-broken';
            break;
        case appModels.HealthStatuses.Progressing:
            color = COLORS.health.progressing;
            icon = 'fa fa-circle-notch fa-spin';
            break;
    }
    let title: string = state.status;
    if (state.message) {
        title = `${state.status}: ${state.message};`;
    }
    return <i title={title} className={'fa ' + icon} style={{color}}/>;
};

export const ResourceResultIcon = ({resource}: { resource: appModels.ResourceResult }) => {
    let color = COLORS.sync_result.unknown;
    let icon = 'fa-question-circle';

    if (!resource.hookType && resource.status) {
        switch (resource.status) {
            case appModels.ResultCodes.Synced:
                color = COLORS.sync_result.synced;
                icon = 'fa-heartbeat';
                break;
            case appModels.ResultCodes.Pruned:
                color = COLORS.sync_result.pruned;
                icon = 'fa-heartbeat';
                break;
            case appModels.ResultCodes.SyncFailed:
                color = COLORS.sync_result.failed;
                icon = 'fa-heart-broken';
                break;
            case appModels.ResultCodes.PruneSkipped:
                icon = 'fa-heartbeat';
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.status}: ${resource.message};`;
        }
        return <i title={title} className={'fa ' + icon} style={{color}}/>;
    }
    if (resource.hookType && resource.hookPhase) {
        let className = '';
        switch (resource.hookPhase) {
            case appModels.OperationPhases.Running:
                color = COLORS.operation.running;
                className = 'fa fa-circle-notch fa-spin';
                break;
            case appModels.OperationPhases.Failed:
                color = COLORS.operation.failed;
                className = 'fa fa-heart-broken';
                break;
            case appModels.OperationPhases.Error:
                color = COLORS.operation.error;
                className = 'fa fa-heart-broken';
                break;
            case appModels.OperationPhases.Succeeded:
                color = COLORS.operation.success;
                className = 'fa fa-heartbeat';
                break;
            case appModels.OperationPhases.Terminating:
                color = COLORS.operation.terminating;
                className = 'fa fa-circle-notch fa-spin';
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.hookPhase}: ${resource.message};`;
        }
        return <i title={title} className={className} style={{color}}/>;
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
            } else if (container.state.terminated && container.state.terminated.reason) {
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

export function isAppNode(node: appModels.ResourceNode) {
    return node.kind === 'Application' && node.group === 'argoproj.io';
}

export function getAppOverridesCount(app: appModels.Application) {
    if (app.spec.source.ksonnet && app.spec.source.ksonnet.parameters) {
        return app.spec.source.ksonnet.parameters.length;
    }
    if (app.spec.source.kustomize && app.spec.source.kustomize.images) {
        return app.spec.source.kustomize.images.length;
    }
    if (app.spec.source.helm && app.spec.source.helm.parameters) {
        return app.spec.source.helm.parameters.length;
    }
    return 0;
}

export function isAppRefreshing(app: appModels.Application) {
    return !!(app.metadata.annotations && app.metadata.annotations[appModels.AnnotationRefreshKey]);
}

export function refreshLinkAttrs(app: appModels.Application) {
    return { disabled: isAppRefreshing(app) };
}
