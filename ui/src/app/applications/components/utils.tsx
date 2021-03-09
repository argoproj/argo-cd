import {DataLoader, FormField, MenuItem, NotificationType, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Checkbox, Text} from 'react-form';
import {BehaviorSubject, Observable, Observer, Subscription} from 'rxjs';
import {AppContext} from '../../shared/context';
import {ResourceTreeNode} from './application-resource-tree/application-resource-tree';

import {COLORS, ErrorNotification, Revision} from '../../shared/components';
import {ContextApis} from '../../shared/context';
import * as appModels from '../../shared/models';
import {services} from '../../shared/services';

export interface NodeId {
    kind: string;
    namespace: string;
    name: string;
    group: string;
}

type ActionMenuItem = MenuItem & {disabled?: boolean};

export function nodeKey(node: NodeId) {
    return [node.group, node.kind, node.namespace, node.name].join('/');
}

export function isSameNode(first: NodeId, second: NodeId) {
    return nodeKey(first) === nodeKey(second);
}

export function helpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}>
                {' '}
                <i className='fas fa-info-circle' />
            </span>
        </Tooltip>
    );
}
export async function deleteApplication(appName: string, apis: ContextApis): Promise<boolean> {
    let confirmed = false;
    await apis.popup.prompt(
        'Delete application',
        api => (
            <div>
                <p>Are you sure you want to delete the application '{appName}'?</p>
                <div className='argo-form-row'>
                    <FormField
                        label={`Please type '${appName}' to confirm the deletion of the resource`}
                        formApi={api}
                        field='applicationName'
                        qeId='name-field-delete-confirmation'
                        component={Text}
                    />
                </div>
                <div className='argo-form-row'>
                    <Checkbox id='cascade-checkbox-delete-confirmation' field='cascadeCheckbox' /> <label htmlFor='cascade-checkbox-delete-confirmation'>Cascade</label>
                </div>
            </div>
        ),
        {
            validate: vals => ({
                applicationName: vals.applicationName !== appName && 'Enter the application name to confirm the deletion'
            }),
            submit: async (vals, _, close) => {
                try {
                    await services.applications.delete(appName, vals.cascadeCheckbox);
                    confirmed = true;
                    close();
                } catch (e) {
                    apis.notifications.show({
                        content: <ErrorNotification title='Unable to delete application' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        },
        {name: 'argo-icon-warning', color: 'warning'},
        'yellow',
        {cascadeCheckbox: true}
    );
    return confirmed;
}

export const OperationPhaseIcon = ({app}: {app: appModels.Application}) => {
    const operationState = getAppOperationState(app);
    if (operationState === undefined) {
        return <React.Fragment />;
    }
    let className = '';
    let color = '';
    switch (operationState.phase) {
        case appModels.OperationPhases.Succeeded:
            className = 'fa fa-check-circle';
            color = COLORS.operation.success;
            break;
        case appModels.OperationPhases.Error:
            className = 'fa fa-times-circle';
            color = COLORS.operation.error;
            break;
        case appModels.OperationPhases.Failed:
            className = 'fa fa-times-circle';
            color = COLORS.operation.failed;
            break;
        default:
            className = 'fa fa-circle-notch fa-spin';
            color = COLORS.operation.running;
            break;
    }
    return <i title={getOperationStateTitle(app)} qe-id='utils-operations-status-title' className={className} style={{color}} />;
};

export const ComparisonStatusIcon = ({status, resource, label}: {status: appModels.SyncStatusCode; resource?: {requiresPruning?: boolean}; label?: boolean}) => {
    let className = 'fas fa-question-circle';
    let color = COLORS.sync.unknown;
    let title: string = 'Unknown';

    switch (status) {
        case appModels.SyncStatuses.Synced:
            className = 'fa fa-check-circle';
            color = COLORS.sync.synced;
            title = 'Synced';
            break;
        case appModels.SyncStatuses.OutOfSync:
            const requiresPruning = resource && resource.requiresPruning;
            className = requiresPruning ? 'fa fa-times-circle' : 'fa fa-arrow-alt-circle-up';
            title = 'OutOfSync';
            if (requiresPruning) {
                title = `${title} (requires pruning)`;
            }
            color = COLORS.sync.out_of_sync;
            break;
        case appModels.SyncStatuses.Unknown:
            className = 'fa fa-circle-notch fa-spin';
            break;
    }
    return (
        <React.Fragment>
            <i qe-id='utils-sync-status-title' title={title} className={className} style={{color}} /> {label && title}
        </React.Fragment>
    );
};

export function showDeploy(resource: string, appContext: AppContext) {
    appContext.apis.navigation.goto('.', {deploy: resource});
}

export function findChildPod(node: appModels.ResourceNode, tree: appModels.ApplicationTree): appModels.ResourceNode {
    const key = nodeKey(node);

    const allNodes = tree.nodes.concat(tree.orphanedNodes || []);
    const nodeByKey = new Map<string, appModels.ResourceNode>();
    allNodes.forEach(item => nodeByKey.set(nodeKey(item), item));

    const pods = tree.nodes.concat(tree.orphanedNodes || []).filter(item => item.kind === 'Pod');
    return pods.find(pod => {
        const items: Array<appModels.ResourceNode> = [pod];
        while (items.length > 0) {
            const next = items.pop();
            const parentKeys = (next.parentRefs || []).map(nodeKey);
            if (parentKeys.includes(key)) {
                return true;
            }
            parentKeys.forEach(item => {
                const parent = nodeByKey.get(item);
                if (parent) {
                    items.push(parent);
                }
            });
        }

        return false;
    });
}

export function renderResourceMenu(
    resource: ResourceTreeNode,
    application: appModels.Application,
    tree: appModels.ApplicationTree,
    appContext: AppContext,
    appChanged: BehaviorSubject<appModels.Application>,
    getApplicationActionMenu: () => any
): React.ReactNode {
    let menuItems: Observable<ActionMenuItem[]>;
    const deleteOptions = {
        option: 'foreground'
    };
    function handleStateChange(option: string) {
        deleteOptions.option = option;
    }
    if (isAppNode(resource) && resource.name === application.metadata.name) {
        menuItems = Observable.from([getApplicationActionMenu()]);
    } else {
        const isRoot = resource.root && nodeKey(resource.root) === nodeKey(resource);
        const isManaged = !!resource.status;
        const items: MenuItem[] = [
            ...((isRoot && [
                {
                    title: 'Sync',
                    action: () => showDeploy(nodeKey(resource), appContext)
                }
            ]) ||
                []),
            {
                title: 'Delete',
                action: async () => {
                    appContext.apis.popup.prompt(
                        'Delete resource',
                        api => (
                            <div>
                                <p>
                                    Are you sure you want to delete {resource.kind} '{resource.name}'?
                                </p>
                                {isManaged ? (
                                    <div className='argo-form-row'>
                                        <FormField
                                            label={`Please type '${resource.name}' to confirm the deletion of the resource`}
                                            formApi={api}
                                            field='resourceName'
                                            component={Text}
                                        />
                                    </div>
                                ) : (
                                    ''
                                )}
                                <div className='argo-form-row'>
                                    <input
                                        type='radio'
                                        name='deleteOptions'
                                        value='foreground'
                                        onChange={() => handleStateChange('foreground')}
                                        defaultChecked={true}
                                        style={{marginRight: '5px'}}
                                    />
                                    <label htmlFor='foreground-delete-radio' style={{paddingRight: '30px'}}>
                                        Foreground Delete {helpTip('Deletes the resource and dependent resources using the cascading policy in the foreground')}
                                    </label>
                                    <input type='radio' name='deleteOptions' value='force' onChange={() => handleStateChange('force')} style={{marginRight: '5px'}} />
                                    <label htmlFor='force-delete-radio' style={{paddingRight: '30px'}}>
                                        Force Delete {helpTip('Deletes the resource and its dependent resources in the background')}
                                    </label>
                                    <input type='radio' name='deleteOptions' value='orphan' onChange={() => handleStateChange('orphan')} style={{marginRight: '5px'}} />
                                    <label htmlFor='cascade-delete-radio'>
                                        Non-cascading (Orphan) Delete {helpTip('Deletes the resource and orphans the dependent resources')}
                                    </label>
                                </div>
                            </div>
                        ),
                        {
                            validate: vals =>
                                isManaged && {
                                    resourceName: vals.resourceName !== resource.name && 'Enter the resource name to confirm the deletion'
                                },
                            submit: async (vals, _, close) => {
                                const force = deleteOptions.option === 'force';
                                const orphan = deleteOptions.option === 'orphan';
                                try {
                                    await services.applications.deleteResource(application.metadata.name, resource, !!force, !!orphan);
                                    appChanged.next(await services.applications.get(application.metadata.name));
                                    close();
                                } catch (e) {
                                    appContext.apis.notifications.show({
                                        content: <ErrorNotification title='Unable to delete resource' e={e} />,
                                        type: NotificationType.Error
                                    });
                                }
                            }
                        },
                        {name: 'argo-icon-warning', color: 'warning'},
                        'yellow'
                    );
                }
            }
        ];
        if (findChildPod(resource, tree)) {
            items.push({
                title: 'Logs',
                action: () => appContext.apis.navigation.goto('.', {node: nodeKey(resource), tab: 'logs'})
            });
        }
        const resourceActions = services.applications
            .getResourceActions(application.metadata.name, resource)
            .then(actions =>
                items.concat(
                    actions.map(action => ({
                        title: action.name,
                        disabled: !!action.disabled,
                        action: async () => {
                            try {
                                const confirmed = await appContext.apis.popup.confirm(
                                    `Execute '${action.name}' action?`,
                                    `Are you sure you want to execute '${action.name}' action?`
                                );
                                if (confirmed) {
                                    await services.applications.runResourceAction(application.metadata.name, resource, action.name);
                                }
                            } catch (e) {
                                appContext.apis.notifications.show({
                                    content: <ErrorNotification title='Unable to execute resource action' e={e} />,
                                    type: NotificationType.Error
                                });
                            }
                        }
                    }))
                )
            )
            .catch(() => items);
        menuItems = Observable.merge(Observable.from([items]), Observable.fromPromise(resourceActions));
    }
    return (
        <DataLoader load={() => menuItems}>
            {items => (
                <ul>
                    {items.map((item, i) => (
                        <li
                            className={classNames('application-details__action-menu', {disabled: item.disabled})}
                            key={i}
                            onClick={e => {
                                e.stopPropagation();
                                if (!item.disabled) {
                                    item.action();
                                    document.body.click();
                                }
                            }}>
                            {item.iconClassName && <i className={item.iconClassName} />} {item.title}
                        </li>
                    ))}
                </ul>
            )}
        </DataLoader>
    );
}

export function syncStatusMessage(app: appModels.Application) {
    const rev = app.status.sync.revision || app.spec.source.targetRevision || 'HEAD';
    let message = app.spec.source.targetRevision || 'HEAD';
    if (app.status.sync.revision) {
        if (app.spec.source.chart) {
            message += ' (' + app.status.sync.revision + ')';
        } else if (app.status.sync.revision.length >= 7 && !app.status.sync.revision.startsWith(app.spec.source.targetRevision)) {
            message += ' (' + app.status.sync.revision.substr(0, 7) + ')';
        }
    }
    switch (app.status.sync.status) {
        case appModels.SyncStatuses.Synced:
            return (
                <span>
                    To{' '}
                    <Revision repoUrl={app.spec.source.repoURL} revision={rev}>
                        {message}
                    </Revision>{' '}
                </span>
            );
        case appModels.SyncStatuses.OutOfSync:
            return (
                <span>
                    From{' '}
                    <Revision repoUrl={app.spec.source.repoURL} revision={rev}>
                        {message}
                    </Revision>{' '}
                </span>
            );
        default:
            return <span>{message}</span>;
    }
}

export const HealthStatusIcon = ({state}: {state: appModels.HealthStatus}) => {
    let color = COLORS.health.unknown;
    let icon = 'fa-question-circle';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            color = COLORS.health.healthy;
            icon = 'fa-heart';
            break;
        case appModels.HealthStatuses.Suspended:
            color = COLORS.health.suspended;
            icon = 'fa-pause-circle';
            break;
        case appModels.HealthStatuses.Degraded:
            color = COLORS.health.degraded;
            icon = 'fa-heart-broken';
            break;
        case appModels.HealthStatuses.Progressing:
            color = COLORS.health.progressing;
            icon = 'fa fa-circle-notch fa-spin';
            break;
        case appModels.HealthStatuses.Missing:
            color = COLORS.health.missing;
            icon = 'fa-ghost';
            break;
    }
    let title: string = state.status;
    if (state.message) {
        title = `${state.status}: ${state.message}`;
    }
    return <i qe-id='utils-health-status-title' title={title} className={'fa ' + icon} style={{color}} />;
};

export const PodHealthIcon = ({state}: {state: appModels.HealthStatus}) => {
    let icon = 'fa-question-circle';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            icon = 'fa-check';
            break;
        case appModels.HealthStatuses.Suspended:
            icon = 'fa-check';
            break;
        case appModels.HealthStatuses.Degraded:
            icon = 'fa-times';
            break;
        case appModels.HealthStatuses.Progressing:
            icon = 'fa fa-circle-notch fa-spin';
            break;
    }
    let title: string = state.status;
    if (state.message) {
        title = `${state.status}: ${state.message}`;
    }
    return <i qe-id='utils-health-status-title' title={title} className={'fa ' + icon} />;
};

export const PodPhaseIcon = ({state}: {state: appModels.PodPhase}) => {
    let className = '';
    switch (state) {
        case appModels.PodPhase.PodSucceeded:
            className = 'fa fa-check';
            break;
        case appModels.PodPhase.PodRunning:
            className = 'fa fa-circle-notch fa-spin';
            break;
        case appModels.PodPhase.PodPending:
            className = 'fa fa-circle-notch fa-spin';
            break;
        case appModels.PodPhase.PodFailed:
            className = 'fa fa-times';
            break;
        default:
            className = 'fa fa-question-circle';
            break;
    }
    return <i qe-id='utils-pod-phase-icon' className={className} />;
};

export const ResourceResultIcon = ({resource}: {resource: appModels.ResourceResult}) => {
    let color = COLORS.sync_result.unknown;
    let icon = 'fas fa-question-circle';

    if (!resource.hookType && resource.status) {
        switch (resource.status) {
            case appModels.ResultCodes.Synced:
                color = COLORS.sync_result.synced;
                icon = 'fa-heart';
                break;
            case appModels.ResultCodes.Pruned:
                color = COLORS.sync_result.pruned;
                icon = 'fa-heart';
                break;
            case appModels.ResultCodes.SyncFailed:
                color = COLORS.sync_result.failed;
                icon = 'fa-heart-broken';
                break;
            case appModels.ResultCodes.PruneSkipped:
                icon = 'fa-heart';
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.status}: ${resource.message}`;
        }
        return <i title={title} className={'fa ' + icon} style={{color}} />;
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
                className = 'fa fa-heart';
                break;
            case appModels.OperationPhases.Terminating:
                color = COLORS.operation.terminating;
                className = 'fa fa-circle-notch fa-spin';
                break;
        }
        let title: string = resource.message;
        if (resource.message) {
            title = `${resource.hookPhase}: ${resource.message}`;
        }
        return <i title={title} className={className} style={{color}} />;
    }
    return null;
};

export const getAppOperationState = (app: appModels.Application): appModels.OperationState => {
    if (app.operation) {
        return {
            phase: appModels.OperationPhases.Running,
            message: (app.status && app.status.operationState && app.status.operationState.message) || 'waiting to start',
            startedAt: new Date().toISOString(),
            operation: {
                sync: {}
            }
        } as appModels.OperationState;
    } else if (app.metadata.deletionTimestamp) {
        return {
            phase: appModels.OperationPhases.Running,
            startedAt: app.metadata.deletionTimestamp
        } as appModels.OperationState;
    } else {
        return app.status.operationState;
    }
};

export function getOperationType(application: appModels.Application) {
    const operation = application.operation || (application.status && application.status.operationState && application.status.operationState.operation);
    if (operation && operation.sync) {
        return 'Sync';
    }
    if (application.metadata.deletionTimestamp) {
        return 'Delete';
    }
    return 'Unknown';
}

const getOperationStateTitle = (app: appModels.Application) => {
    const appOperationState = getAppOperationState(app);
    const operationType = getOperationType(app);
    switch (operationType) {
        case 'Delete':
            return 'Deleting';
        case 'Sync':
            switch (appOperationState.phase) {
                case 'Running':
                    return 'Syncing';
                case 'Error':
                    return 'Sync error';
                case 'Failed':
                    return 'Sync failed';
                case 'Succeeded':
                    return 'Sync OK';
                case 'Terminating':
                    return 'Terminated';
            }
    }
    return 'Unknown';
};

export const OperationState = ({app, quiet}: {app: appModels.Application; quiet?: boolean}) => {
    const appOperationState = getAppOperationState(app);
    if (appOperationState === undefined) {
        return <React.Fragment />;
    }
    if (quiet && [appModels.OperationPhases.Running, appModels.OperationPhases.Failed, appModels.OperationPhases.Error].indexOf(appOperationState.phase) === -1) {
        return <React.Fragment />;
    }

    return (
        <React.Fragment>
            <OperationPhaseIcon app={app} /> {getOperationStateTitle(app)}
        </React.Fragment>
    );
};

export function getPodStateReason(pod: appModels.State): {message: string; reason: string} {
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
        for (const container of pod.status.containerStatuses || []) {
            if (container.state.waiting && container.state.waiting.reason) {
                reason = container.state.waiting.reason;
                message = container.state.waiting.message;
            } else if (container.state.terminated && container.state.terminated.reason) {
                reason = container.state.terminated.reason;
                message = container.state.terminated.message;
            } else if (container.state.terminated && !container.state.terminated.reason) {
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

        // change pod status back to 'Running' if there is at least one container still reporting as 'Running' status
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

export function setAppRefreshing(app: appModels.Application) {
    if (!app.metadata.annotations) {
        app.metadata.annotations = {};
    }
    if (!app.metadata.annotations[appModels.AnnotationRefreshKey]) {
        app.metadata.annotations[appModels.AnnotationRefreshKey] = 'refreshing';
    }
}

export function refreshLinkAttrs(app: appModels.Application) {
    return {disabled: isAppRefreshing(app)};
}

export const SyncWindowStatusIcon = ({state, window}: {state: appModels.SyncWindowsState; window: appModels.SyncWindow}) => {
    let className = '';
    let color = '';
    let current = '';

    if (state.windows === undefined) {
        current = 'Inactive';
    } else {
        for (const w of state.windows) {
            if (w.kind === window.kind && w.schedule === window.schedule && w.duration === window.duration) {
                current = 'Active';
                break;
            } else {
                current = 'Inactive';
            }
        }
    }

    switch (current + ':' + window.kind) {
        case 'Active:deny':
        case 'Inactive:allow':
            className = 'fa fa-stop-circle';
            if (window.manualSync) {
                color = COLORS.sync_window.manual;
            } else {
                color = COLORS.sync_window.deny;
            }
            break;
        case 'Active:allow':
        case 'Inactive:deny':
            className = 'fa fa-check-circle';
            color = COLORS.sync_window.allow;
            break;
        default:
            className = 'fas fa-question-circle';
            color = COLORS.sync_window.unknown;
            current = 'Unknown';
            break;
    }

    return (
        <React.Fragment>
            <i title={current} className={className} style={{color}} /> {current}
        </React.Fragment>
    );
};

export const ApplicationSyncWindowStatusIcon = ({project, state}: {project: string; state: appModels.ApplicationSyncWindowState}) => {
    let className = '';
    let color = '';
    let deny = false;
    let allow = false;
    let inactiveAllow = false;
    if (state.assignedWindows !== undefined && state.assignedWindows.length > 0) {
        if (state.activeWindows !== undefined && state.activeWindows.length > 0) {
            for (const w of state.activeWindows) {
                if (w.kind === 'deny') {
                    deny = true;
                } else if (w.kind === 'allow') {
                    allow = true;
                }
            }
        }
        for (const a of state.assignedWindows) {
            if (a.kind === 'allow') {
                inactiveAllow = true;
            }
        }
    } else {
        allow = true;
    }

    if (deny || (!deny && !allow && inactiveAllow)) {
        className = 'fa fa-stop-circle';
        if (state.canSync) {
            color = COLORS.sync_window.manual;
        } else {
            color = COLORS.sync_window.deny;
        }
    } else {
        className = 'fa fa-check-circle';
        color = COLORS.sync_window.allow;
    }

    return (
        <a href={`/settings/projects/${project}?tab=windows`} style={{color}}>
            <i className={className} style={{color}} /> SyncWindow
        </a>
    );
};

/**
 * Automatically stops and restarts the given observable when page visibility changes.
 */
export function handlePageVisibility<T>(src: () => Observable<T>): Observable<T> {
    return new Observable<T>((observer: Observer<T>) => {
        let subscription: Subscription;
        const ensureUnsubscribed = () => {
            if (subscription) {
                subscription.unsubscribe();
                subscription = null;
            }
        };
        const start = () => {
            ensureUnsubscribed();
            subscription = src().subscribe((item: T) => observer.next(item), err => observer.error(err), () => observer.complete());
        };

        if (!document.hidden) {
            start();
        }

        const visibilityChangeSubscription = Observable.fromEvent(document, 'visibilitychange')
            // wait until user stop clicking back and forth to avoid restarting observable too often
            .debounceTime(500)
            .subscribe(() => {
                if (document.hidden && subscription) {
                    ensureUnsubscribed();
                } else if (!document.hidden && !subscription) {
                    start();
                }
            });

        return () => {
            visibilityChangeSubscription.unsubscribe();
            ensureUnsubscribed();
        };
    });
}

export function parseApiVersion(apiVersion: string): {group: string; version: string} {
    const parts = apiVersion.split('/');
    if (parts.length > 1) {
        return {group: parts[0], version: parts[1]};
    }
    return {version: parts[0], group: ''};
}

export function getContainerName(pod: any, containerIndex: number): string {
    const containers = (pod.spec.containers || []).concat(pod.spec.initContainers || []);
    const container = containers[containerIndex];
    return container.name;
}

export const BASE_COLORS = [
    '#0DADEA', // blue
    '#95D58F', // green
    '#F4C030', // orange
    '#FF6262', // red
    '#4B0082', // purple
    '#964B00' // brown
];
