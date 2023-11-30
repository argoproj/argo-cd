import {models, DataLoader, FormField, MenuItem, NotificationType, Tooltip} from 'argo-ui';
import {ActionButton} from 'argo-ui/v2';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';
import {FormApi, Text} from 'react-form';
import * as moment from 'moment';
import {BehaviorSubject, combineLatest, concat, from, fromEvent, Observable, Observer, Subscription} from 'rxjs';
import {debounceTime, map} from 'rxjs/operators';
import {AppContext, Context, ContextApis} from '../../shared/context';
import {ResourceTreeNode} from './application-resource-tree/application-resource-tree';

import {CheckboxField, COLORS, ErrorNotification, Revision} from '../../shared/components';
import * as appModels from '../../shared/models';
import {services} from '../../shared/services';

require('./utils.scss');

export interface NodeId {
    kind: string;
    namespace: string;
    name: string;
    group: string;
    createdAt?: models.Time;
}

type ActionMenuItem = MenuItem & {disabled?: boolean; tooltip?: string};

export function nodeKey(node: NodeId) {
    return [node.group, node.kind, node.namespace, node.name].join('/');
}

export function createdOrNodeKey(node: NodeId) {
    return node?.createdAt || nodeKey(node);
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
export async function deleteApplication(appName: string, appNamespace: string, apis: ContextApis): Promise<boolean> {
    let confirmed = false;
    const propagationPolicies: {name: string; message: string}[] = [
        {
            name: 'Foreground',
            message: `Cascade delete the application's resources using foreground propagation policy`
        },
        {
            name: 'Background',
            message: `Cascade delete the application's resources using background propagation policy`
        },
        {
            name: 'Non-cascading',
            message: `Only delete the application, but do not cascade delete its resources`
        }
    ];
    await apis.popup.prompt(
        'Delete application',
        api => (
            <div>
                <p>
                    Are you sure you want to delete the application <kbd>{appName}</kbd>?
                </p>
                <div className='argo-form-row'>
                    <FormField
                        label={`Please type '${appName}' to confirm the deletion of the resource`}
                        formApi={api}
                        field='applicationName'
                        qeId='name-field-delete-confirmation'
                        component={Text}
                    />
                </div>
                <p>Select propagation policy for application deletion</p>
                <div className='propagation-policy-list'>
                    {propagationPolicies.map(policy => {
                        return (
                            <FormField
                                formApi={api}
                                key={policy.name}
                                field='propagationPolicy'
                                component={PropagationPolicyOption}
                                componentProps={{
                                    policy: policy.name,
                                    message: policy.message
                                }}
                            />
                        );
                    })}
                </div>
            </div>
        ),
        {
            validate: vals => ({
                applicationName: vals.applicationName !== appName && 'Enter the application name to confirm the deletion'
            }),
            submit: async (vals, _, close) => {
                try {
                    await services.applications.delete(appName, appNamespace, vals.propagationPolicy);
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
        {propagationPolicy: 'foreground'}
    );
    return confirmed;
}

export async function confirmSyncingAppOfApps(apps: appModels.Application[], apis: ContextApis, form: FormApi): Promise<boolean> {
    let confirmed = false;
    const appNames: string[] = apps.map(app => app.metadata.name);
    const appNameList = appNames.join(', ');
    await apis.popup.prompt(
        'Warning: Synchronize App of Multiple Apps using replace?',
        api => (
            <div>
                <p>
                    Are you sure you want to sync the application '{appNameList}' which contain(s) multiple apps with 'replace' option? This action will delete and recreate all
                    apps linked to '{appNameList}'.
                </p>
                <div className='argo-form-row'>
                    <FormField
                        label={`Please type '${appNameList}' to confirm the Syncing of the resource`}
                        formApi={api}
                        field='applicationName'
                        qeId='name-field-delete-confirmation'
                        component={Text}
                    />
                </div>
            </div>
        ),
        {
            validate: vals => ({
                applicationName: vals.applicationName !== appNameList && 'Enter the application name(s) to confirm syncing'
            }),
            submit: async (_vals, _, close) => {
                try {
                    await form.submitForm(null);
                    confirmed = true;
                    close();
                } catch (e) {
                    apis.notifications.show({
                        content: <ErrorNotification title='Unable to sync application' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        },
        {name: 'argo-icon-warning', color: 'warning'},
        'yellow'
    );
    return confirmed;
}

const PropagationPolicyOption = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi; policy: string; message: string}) => {
    const {
        fieldApi: {setValue}
    } = props;
    return (
        <div className='propagation-policy-option'>
            <input
                className='radio-button'
                key={props.policy}
                type='radio'
                name='propagation-policy'
                value={props.policy}
                id={props.policy}
                defaultChecked={props.policy === 'Foreground'}
                onChange={() => setValue(props.policy.toLowerCase())}
            />
            <label htmlFor={props.policy}>
                {props.policy} {helpTip(props.message)}
            </label>
        </div>
    );
});

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

export const ComparisonStatusIcon = ({
    status,
    resource,
    label,
    noSpin
}: {
    status: appModels.SyncStatusCode;
    resource?: {requiresPruning?: boolean};
    label?: boolean;
    noSpin?: boolean;
}) => {
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
            className = requiresPruning ? 'fa fa-trash' : 'fa fa-arrow-alt-circle-up';
            title = 'OutOfSync';
            if (requiresPruning) {
                title = `${title} (This resource is not present in the application's source. It will be deleted from Kubernetes if the prune option is enabled during sync.)`;
            }
            color = COLORS.sync.out_of_sync;
            break;
        case appModels.SyncStatuses.Unknown:
            className = `fa fa-circle-notch ${noSpin ? '' : 'fa-spin'}`;
            break;
    }
    return (
        <React.Fragment>
            <i qe-id='utils-sync-status-title' title={title} className={className} style={{color}} /> {label && title}
        </React.Fragment>
    );
};

export function showDeploy(resource: string, revision: string, apis: ContextApis) {
    apis.navigation.goto('.', {deploy: resource, revision}, {replace: true});
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

export const deletePodAction = async (pod: appModels.Pod, appContext: AppContext, appName: string, appNamespace: string) => {
    appContext.apis.popup.prompt(
        'Delete pod',
        () => (
            <div>
                <p>
                    Are you sure you want to delete Pod <kbd>{pod.name}</kbd>?
                </p>
                <div className='argo-form-row' style={{paddingLeft: '30px'}}>
                    <CheckboxField id='force-delete-checkbox' field='force'>
                        <label htmlFor='force-delete-checkbox'>Force delete</label>
                    </CheckboxField>
                </div>
            </div>
        ),
        {
            submit: async (vals, _, close) => {
                try {
                    await services.applications.deleteResource(appName, appNamespace, pod, !!vals.force, false);
                    close();
                } catch (e) {
                    appContext.apis.notifications.show({
                        content: <ErrorNotification title='Unable to delete resource' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        }
    );
};

export const deletePopup = async (ctx: ContextApis, resource: ResourceTreeNode, application: appModels.Application, appChanged?: BehaviorSubject<appModels.Application>) => {
    const isManaged = !!resource.status;
    const deleteOptions = {
        option: 'foreground'
    };
    function handleStateChange(option: string) {
        deleteOptions.option = option;
    }
    return ctx.popup.prompt(
        'Delete resource',
        api => (
            <div>
                <p>
                    Are you sure you want to delete {resource.kind} <kbd>{resource.name}</kbd>?
                </p>
                {isManaged ? (
                    <div className='argo-form-row'>
                        <FormField label={`Please type '${resource.name}' to confirm the deletion of the resource`} formApi={api} field='resourceName' component={Text} />
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
                        id='foreground-delete-radio'
                    />
                    <label htmlFor='foreground-delete-radio' style={{paddingRight: '30px'}}>
                        Foreground Delete {helpTip('Deletes the resource and dependent resources using the cascading policy in the foreground')}
                    </label>
                    <input type='radio' name='deleteOptions' value='force' onChange={() => handleStateChange('force')} style={{marginRight: '5px'}} id='force-delete-radio' />
                    <label htmlFor='force-delete-radio' style={{paddingRight: '30px'}}>
                        Force Delete {helpTip('Deletes the resource and its dependent resources in the background')}
                    </label>
                    <input type='radio' name='deleteOptions' value='orphan' onChange={() => handleStateChange('orphan')} style={{marginRight: '5px'}} id='cascade-delete-radio' />
                    <label htmlFor='cascade-delete-radio'>Non-cascading (Orphan) Delete {helpTip('Deletes the resource and orphans the dependent resources')}</label>
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
                    await services.applications.deleteResource(application.metadata.name, application.metadata.namespace, resource, !!force, !!orphan);
                    if (appChanged) {
                        appChanged.next(await services.applications.get(application.metadata.name, application.metadata.namespace));
                    }
                    close();
                } catch (e) {
                    ctx.notifications.show({
                        content: <ErrorNotification title='Unable to delete resource' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        },
        {name: 'argo-icon-warning', color: 'warning'},
        'yellow'
    );
};

function getResourceActionsMenuItems(resource: ResourceTreeNode, metadata: models.ObjectMeta, apis: ContextApis): Promise<ActionMenuItem[]> {
    return services.applications.getResourceActions(metadata.name, metadata.namespace, resource).then(actions => {
        return actions.map(action => ({
            title: action.displayName ?? action.name,
            disabled: !!action.disabled,
            iconClassName: action.iconClass,
            action: async () => {
                const confirmed = false;
                const title = action.hasParameters ? `Enter input parameters for action: ${action.name}` : `Execute ${action.name} action?`;
                await apis.popup.prompt(
                    title,
                    api => (
                        <div>
                            {!action.hasParameters && (
                                <div className='argo-form-row'>
                                    <div> Are you sure you want to execute {action.name} action?</div>
                                </div>
                            )}
                            {action.hasParameters && (
                                <div className='argo-form-row'>
                                    <FormField formApi={api} field='inputParameter' component={Text} componentProps={{showErrors: true}} />
                                </div>
                            )}
                        </div>
                    ),
                    {
                        validate: vals => ({
                            inputParameter: !vals.inputParameter.match(action.regexp) && action.errorMessage
                        }),
                        submit: async (vals, _, close) => {
                            try {
                                const resourceActionParameters = action.hasParameters ? [{name: action.name, value: vals.inputParameter}] : [];
                                await services.applications.runResourceAction(metadata.name, metadata.namespace, resource, action.name, resourceActionParameters);
                                close();
                            } catch (e) {
                                apis.notifications.show({
                                    content: <ErrorNotification title='Unable to run action' e={e} />,
                                    type: NotificationType.Error
                                });
                            }
                        }
                    },
                    null,
                    null,
                    {inputParameter: action.defaultValue}
                );
                return confirmed;
            }
        }));
    });
}

function getActionItems(
    resource: ResourceTreeNode,
    application: appModels.Application,
    tree: appModels.ApplicationTree,
    apis: ContextApis,
    appChanged: BehaviorSubject<appModels.Application>,
    isQuickStart: boolean
): Observable<ActionMenuItem[]> {
    const isRoot = resource.root && nodeKey(resource.root) === nodeKey(resource);
    const items: MenuItem[] = [
        ...((isRoot && [
            {
                title: 'Sync',
                iconClassName: 'fa fa-fw fa-sync',
                action: () => showDeploy(nodeKey(resource), null, apis)
            }
        ]) ||
            []),
        {
            title: 'Delete',
            iconClassName: 'fa fa-fw fa-times-circle',
            action: async () => {
                return deletePopup(apis, resource, application, appChanged);
            }
        }
    ];
    if (!isQuickStart) {
        items.unshift({
            title: 'Details',
            iconClassName: 'fa fa-fw fa-info-circle',
            action: () => apis.navigation.goto('.', {node: nodeKey(resource)})
        });
    }

    if (findChildPod(resource, tree)) {
        items.push({
            title: 'Logs',
            iconClassName: 'fa fa-fw fa-align-left',
            action: () => apis.navigation.goto('.', {node: nodeKey(resource), tab: 'logs'}, {replace: true})
        });
    }

    if (isQuickStart) {
        return from([items]);
    }

    const execAction = services.authService
        .settings()
        .then(async settings => {
            const execAllowed = await services.accounts.canI('exec', 'create', application.spec.project + '/' + application.metadata.name);
            if (resource.kind === 'Pod' && settings.execEnabled && execAllowed) {
                return [
                    {
                        title: 'Exec',
                        iconClassName: 'fa fa-fw fa-terminal',
                        action: async () => apis.navigation.goto('.', {node: nodeKey(resource), tab: 'exec'}, {replace: true})
                    } as MenuItem
                ];
            }
            return [] as MenuItem[];
        })
        .catch(() => [] as MenuItem[]);

    const resourceActions = getResourceActionsMenuItems(resource, application.metadata, apis);

    const links = services.applications
        .getResourceLinks(application.metadata.name, application.metadata.namespace, resource)
        .then(data => {
            return (data.items || []).map(
                link =>
                    ({
                        title: link.title,
                        iconClassName: `fa fa-fw ${link.iconClass ? link.iconClass : 'fa-external-link'}`,
                        action: () => window.open(link.url, '_blank'),
                        tooltip: link.description
                    } as MenuItem)
            );
        })
        .catch(() => [] as MenuItem[]);

    return combineLatest(
        from([items]), // this resolves immediately
        concat([[] as MenuItem[]], resourceActions), // this resolves at first to [] and then whatever the API returns
        concat([[] as MenuItem[]], execAction), // this resolves at first to [] and then whatever the API returns
        concat([[] as MenuItem[]], links) // this resolves at first to [] and then whatever the API returns
    ).pipe(map(res => ([] as MenuItem[]).concat(...res)));
}

export function renderResourceMenu(
    resource: ResourceTreeNode,
    application: appModels.Application,
    tree: appModels.ApplicationTree,
    apis: ContextApis,
    appChanged: BehaviorSubject<appModels.Application>,
    getApplicationActionMenu: () => any
): React.ReactNode {
    let menuItems: Observable<ActionMenuItem[]>;

    if (isAppNode(resource) && resource.name === application.metadata.name) {
        menuItems = from([getApplicationActionMenu()]);
    } else {
        menuItems = getActionItems(resource, application, tree, apis, appChanged, false);
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
                            {item.tooltip ? (
                                <Tooltip content={item.tooltip || ''}>
                                    <div>
                                        {item.iconClassName && <i className={item.iconClassName} />} {item.title}
                                    </div>
                                </Tooltip>
                            ) : (
                                <>
                                    {item.iconClassName && <i className={item.iconClassName} />} {item.title}
                                </>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </DataLoader>
    );
}

export function renderResourceActionMenu(resource: ResourceTreeNode, application: appModels.Application, apis: ContextApis): React.ReactNode {
    const menuItems = getResourceActionsMenuItems(resource, application.metadata, apis);

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

export function renderResourceButtons(
    resource: ResourceTreeNode,
    application: appModels.Application,
    tree: appModels.ApplicationTree,
    apis: ContextApis,
    appChanged: BehaviorSubject<appModels.Application>
): React.ReactNode {
    let menuItems: Observable<ActionMenuItem[]>;
    menuItems = getActionItems(resource, application, tree, apis, appChanged, true);
    return (
        <DataLoader load={() => menuItems}>
            {items => (
                <div className='pod-view__node__quick-start-actions'>
                    {items.map((item, i) => (
                        <ActionButton
                            disabled={item.disabled}
                            key={i}
                            action={(e: React.MouseEvent) => {
                                e.stopPropagation();
                                if (!item.disabled) {
                                    item.action();
                                    document.body.click();
                                }
                            }}
                            icon={item.iconClassName}
                            tooltip={
                                item.title
                                    .toString()
                                    .charAt(0)
                                    .toUpperCase() + item.title.toString().slice(1)
                            }
                        />
                    ))}
                </div>
            )}
        </DataLoader>
    );
}

export function syncStatusMessage(app: appModels.Application) {
    const source = getAppDefaultSource(app);
    const rev = app.status.sync.revision || source.targetRevision || 'HEAD';
    let message = source.targetRevision || 'HEAD';

    if (app.status.sync.revision) {
        if (source.chart) {
            message += ' (' + app.status.sync.revision + ')';
        } else if (app.status.sync.revision.length >= 7 && !app.status.sync.revision.startsWith(source.targetRevision)) {
            message += ' (' + app.status.sync.revision.substr(0, 7) + ')';
        }
    }
    switch (app.status.sync.status) {
        case appModels.SyncStatuses.Synced:
            return (
                <span>
                    to{' '}
                    <Revision repoUrl={source.repoURL} revision={rev}>
                        {message}
                    </Revision>{' '}
                </span>
            );
        case appModels.SyncStatuses.OutOfSync:
            return (
                <span>
                    from{' '}
                    <Revision repoUrl={source.repoURL} revision={rev}>
                        {message}
                    </Revision>{' '}
                </span>
            );
        default:
            return <span>{message}</span>;
    }
}

export const HealthStatusIcon = ({state, noSpin}: {state: appModels.HealthStatus; noSpin?: boolean}) => {
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
            icon = `fa fa-circle-notch ${noSpin ? '' : 'fa-spin'}`;
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
    if (application.metadata.deletionTimestamp && !application.operation) {
        return 'Delete';
    }
    if (operation && operation.sync) {
        return 'Sync';
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

export function getPodStateReason(pod: appModels.State): {message: string; reason: string; netContainerStatuses: any[]} {
    let reason = pod.status.phase;
    let message = '';
    if (pod.status.reason) {
        reason = pod.status.reason;
    }

    let initializing = false;

    let netContainerStatuses = pod.status.initContainerStatuses || [];
    netContainerStatuses = netContainerStatuses.concat(pod.status.containerStatuses || []);

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

    return {reason, message, netContainerStatuses};
}

export const getPodReadinessGatesState = (pod: appModels.State): {nonExistingConditions: string[]; notPassedConditions: string[]} => {
    // if pod does not have readiness gates then return empty status
    if (!pod.spec?.readinessGates?.length) {
        return {
            nonExistingConditions: [],
            notPassedConditions: []
        };
    }

    const existingConditions = new Map<string, boolean>();
    const podConditions = new Map<string, boolean>();

    const podStatusConditions = pod.status?.conditions || [];

    for (const condition of podStatusConditions) {
        existingConditions.set(condition.type, true);
        // priority order of conditions
        // eg. if there are multiple conditions set with same name then the one which comes first is evaluated
        if (podConditions.has(condition.type)) {
            continue;
        }

        if (condition.status === 'False') {
            podConditions.set(condition.type, false);
        } else if (condition.status === 'True') {
            podConditions.set(condition.type, true);
        }
    }

    const nonExistingConditions: string[] = [];
    const failedConditions: string[] = [];

    const readinessGates: appModels.ReadinessGate[] = pod.spec?.readinessGates || [];

    for (const readinessGate of readinessGates) {
        if (!existingConditions.has(readinessGate.conditionType)) {
            nonExistingConditions.push(readinessGate.conditionType);
        } else if (podConditions.get(readinessGate.conditionType) === false) {
            failedConditions.push(readinessGate.conditionType);
        }
    }

    return {
        nonExistingConditions,
        notPassedConditions: failedConditions
    };
};

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
    const source = getAppDefaultSource(app);
    if (source.kustomize && source.kustomize.images) {
        return source.kustomize.images.length;
    }
    if (source.helm && source.helm.parameters) {
        return source.helm.parameters.length;
    }
    return 0;
}

// getAppDefaultSource gets the first app source from `sources` or, if that list is missing or empty, the `source`
// field.
export function getAppDefaultSource(app?: appModels.Application) {
    if (!app) {
        return null;
    }
    return app.spec.sources && app.spec.sources.length > 0 ? app.spec.sources[0] : app.spec.source;
}

export function getAppSpecDefaultSource(spec: appModels.ApplicationSpec) {
    return spec.sources && spec.sources.length > 0 ? spec.sources[0] : spec.source;
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
            if (w.kind === window.kind && w.schedule === window.schedule && w.duration === window.duration && w.timeZone === window.timeZone) {
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

    const ctx = React.useContext(Context);

    return (
        <a href={`${ctx.baseHref}settings/projects/${project}?tab=windows`} style={{color}}>
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
            subscription = src().subscribe(
                (item: T) => observer.next(item),
                err => observer.error(err),
                () => observer.complete()
            );
        };

        if (!document.hidden) {
            start();
        }

        const visibilityChangeSubscription = fromEvent(document, 'visibilitychange')
            // wait until user stop clicking back and forth to avoid restarting observable too often
            .pipe(debounceTime(500))
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

export function getContainerName(pod: any, containerIndex: number | null): string {
    if (containerIndex == null && pod.metadata?.annotations?.['kubectl.kubernetes.io/default-container']) {
        return pod.metadata?.annotations?.['kubectl.kubernetes.io/default-container'];
    }
    const containers = (pod.spec.containers || []).concat(pod.spec.initContainers || []);
    const container = containers[containerIndex || 0];
    return container.name;
}

export function isYoungerThanXMinutes(pod: any, x: number): boolean {
    const createdAt = moment(pod.createdAt, 'YYYY-MM-DDTHH:mm:ssZ');
    const xMinutesAgo = moment().subtract(x, 'minutes');
    return createdAt.isAfter(xMinutesAgo);
}

export const BASE_COLORS = [
    '#0DADEA', // blue
    '#DE7EAE', // pink
    '#FF9500', // orange
    '#4B0082', // purple
    '#F5d905', // yellow
    '#964B00' // brown
];

export const urlPattern = new RegExp(
    new RegExp(
        // tslint:disable-next-line:max-line-length
        /^(https?:\/\/(?:www\.|(?!www))[a-z0-9][a-z0-9-]+[a-z0-9]\.[^\s]{2,}|www\.[a-z0-9][a-z0-9-]+[a-z0-9]\.[^\s]{2,}|https?:\/\/(?:www\.|(?!www))[a-z0-9]+\.[^\s]{2,}|www\.[a-z0-9]+\.[^\s]{2,})$/,
        'gi'
    )
);

export function appQualifiedName(app: appModels.Application, nsEnabled: boolean): string {
    return (nsEnabled ? app.metadata.namespace + '/' : '') + app.metadata.name;
}

export function appInstanceName(app: appModels.Application): string {
    return app.metadata.namespace + '_' + app.metadata.name;
}

export function formatCreationTimestamp(creationTimestamp: string) {
    const createdAt = moment
        .utc(creationTimestamp)
        .local()
        .format('MM/DD/YYYY HH:mm:ss');
    const fromNow = moment
        .utc(creationTimestamp)
        .local()
        .fromNow();
    return (
        <span>
            {createdAt}
            <i style={{padding: '2px'}} /> ({fromNow})
        </span>
    );
}

export const selectPostfix = (arr: string[], singular: string, plural: string) => (arr.length > 1 ? plural : singular);

export function getUsrMsgKeyToDisplay(appName: string, msgKey: string, usrMessages: appModels.UserMessages[]) {
    const usrMsg = usrMessages?.find((msg: appModels.UserMessages) => msg.appName === appName && msg.msgKey === msgKey);
    if (usrMsg !== undefined) {
        return {...usrMsg, display: true};
    } else {
        return {appName, msgKey, display: false, duration: 1} as appModels.UserMessages;
    }
}

export const userMsgsList: {[key: string]: string} = {
    groupNodes: `Since the number of pods has surpassed the threshold pod count of 15, you will now be switched to the group node view.
                 If you prefer the tree view, you can simply click on the Group Nodes toolbar button to deselect the current view.`
};
