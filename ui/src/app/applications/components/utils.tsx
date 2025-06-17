import {models, DataLoader, FormField, MenuItem, NotificationType, Tooltip, HelpIcon} from 'argo-ui';
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
import {ApplicationSource} from '../../shared/models';

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

//CLassic Solid circle-notch icon
//<!--!Font Awesome Free 6.7.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2025 Fonticons, Inc.-->
//this will replace all <i> fa-spin </i> icons as they are currently misbehaving with no fix available.

export const SpinningIcon = ({color, qeId}: {color: string; qeId: string}) => {
    return (
        <svg className='icon spin' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512' style={{color}} qe-id={qeId}>
            <path
                fill={color}
                d='M222.7 32.1c5 16.9-4.6 34.8-21.5 39.8C121.8 95.6 64 169.1 64 256c0 106 86 192 192 192s192-86 192-192c0-86.9-57.8-160.4-137.1-184.1c-16.9-5-26.6-22.9-21.5-39.8s22.9-26.6 39.8-21.5C434.9 42.1 512 140 512 256c0 141.4-114.6 256-256 256S0 397.4 0 256C0 140 77.1 42.1 182.9 10.6c16.9-5 34.8 4.6 39.8 21.5z'
            />
        </svg>
    );
};

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
                    Are you sure you want to delete the <strong>Application</strong> <kbd>{appName}</kbd>?
                    <span style={{display: 'block', marginBottom: '10px'}} />
                    Deleting the application in <kbd>foreground</kbd> or <kbd>background</kbd> mode will delete all the application's managed resources, which can be{' '}
                    <strong>dangerous</strong>. Be sure you understand the effects of deleting this resource before continuing. Consider asking someone to review the change first.
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
        {name: 'argo-icon-warning', color: 'failed'},
        'red',
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

export const OperationPhaseIcon = ({app, isButton}: {app: appModels.Application; isButton?: boolean}) => {
    const operationState = getAppOperationState(app);
    if (operationState === undefined) {
        return <React.Fragment />;
    }
    let className = '';
    let color = '';
    switch (operationState.phase) {
        case appModels.OperationPhases.Succeeded:
            className = `fa fa-check-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.success;
            break;
        case appModels.OperationPhases.Error:
            className = `fa fa-times-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.error;
            break;
        case appModels.OperationPhases.Failed:
            className = `fa fa-times-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.failed;
            break;
        default:
            className = 'fa fa-circle-notch fa-spin';
            color = COLORS.operation.running;
            break;
    }
    return className.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-operations-status-title' />
    ) : (
        <i title={getOperationStateTitle(app)} qe-id='utils-operations-status-title' className={className} style={{color}} />
    );
};

export const HydrateOperationPhaseIcon = ({operationState, isButton}: {operationState?: appModels.HydrateOperation; isButton?: boolean}) => {
    if (operationState === undefined) {
        return <React.Fragment />;
    }
    let className = '';
    let color = '';
    switch (operationState.phase) {
        case appModels.HydrateOperationPhases.Hydrated:
            className = `fa fa-check-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.success;
            break;
        case appModels.HydrateOperationPhases.Failed:
            className = `fa fa-times-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.failed;
            break;
        default:
            className = 'fa fa-circle-notch fa-spin';
            color = COLORS.operation.running;
            break;
    }
    return className.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-operations-status-title' />
    ) : (
        <i title={operationState.phase} qe-id='utils-operations-status-title' className={className} style={{color}} />
    );
};

export const ComparisonStatusIcon = ({
    status,
    resource,
    label,
    noSpin,
    isButton
}: {
    status: appModels.SyncStatusCode;
    resource?: {requiresPruning?: boolean};
    label?: boolean;
    noSpin?: boolean;
    isButton?: boolean;
}) => {
    let className = 'fas fa-question-circle';
    let color = COLORS.sync.unknown;
    let title: string = 'Unknown';
    switch (status) {
        case appModels.SyncStatuses.Synced:
            className = `fa fa-check-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.sync.synced;
            title = 'Synced';
            break;
        case appModels.SyncStatuses.OutOfSync:
            // eslint-disable-next-line no-case-declarations
            const requiresPruning = resource && resource.requiresPruning;
            className = requiresPruning ? `fa fa-trash${isButton ? ' status-button' : ''}` : `fa fa-arrow-alt-circle-up${isButton ? ' status-button' : ''}`;
            title = 'OutOfSync';
            if (requiresPruning) {
                title = `${title} (This resource is not present in the application's source. It will be deleted from Kubernetes if the prune option is enabled during sync.)`;
            }
            color = COLORS.sync.out_of_sync;
            break;
        case appModels.SyncStatuses.Unknown:
            className = `fa fa-circle-notch ${noSpin ? '' : 'fa-spin'}${isButton ? ' status-button' : ''}`;
            break;
    }
    return className.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-sync-status-title' />
    ) : (
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

export function findChildResources(node: appModels.ResourceNode, tree: appModels.ApplicationTree): appModels.ResourceNode[] {
    const key = nodeKey(node);

    const children: appModels.ResourceNode[] = [];
    tree.nodes.forEach(item => {
        (item.parentRefs || []).forEach(parent => {
            if (key === nodeKey(parent)) {
                children.push(item);
            }
        });
    });

    return children;
}

const deletePodAction = async (ctx: ContextApis, pod: appModels.ResourceNode, app: appModels.Application) => {
    ctx.popup.prompt(
        'Delete pod',
        () => (
            <div>
                <p>
                    Are you sure you want to delete <strong>Pod</strong> <kbd>{pod.name}</kbd>?
                    <span style={{display: 'block', marginBottom: '10px'}} />
                    Deleting resources can be <strong>dangerous</strong>. Be sure you understand the effects of deleting this resource before continuing. Consider asking someone to
                    review the change first.
                </p>
                <div className='argo-form-row' style={{paddingLeft: '30px'}}>
                    <CheckboxField id='force-delete-checkbox' field='force' />
                    <label htmlFor='force-delete-checkbox'>Force delete</label>
                    <HelpIcon title='If checked, Argo will ignore any configured grace period and delete the resource immediately' />
                </div>
            </div>
        ),
        {
            submit: async (vals, _, close) => {
                try {
                    await services.applications.deleteResource(app.metadata.name, app.metadata.namespace, pod, !!vals.force, false);
                    close();
                } catch (e) {
                    ctx.notifications.show({
                        content: <ErrorNotification title='Unable to delete resource' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        }
    );
};

export const deleteSourceAction = (app: appModels.Application, source: appModels.ApplicationSource, appContext: AppContext) => {
    appContext.apis.popup.prompt(
        'Delete source',
        () => (
            <div>
                <p>
                    <>
                        Are you sure you want to delete the source with URL: <kbd>{source.repoURL}</kbd>
                        {source.path ? (
                            <>
                                {' '}
                                and path: <kbd>{source.path}</kbd>?
                            </>
                        ) : (
                            <>?</>
                        )}
                    </>
                </p>
            </div>
        ),
        {
            submit: async (vals, _, close) => {
                try {
                    const i = app.spec.sources.indexOf(source);
                    app.spec.sources.splice(i, 1);
                    await services.applications.update(app);
                    close();
                } catch (e) {
                    appContext.apis.notifications.show({
                        content: <ErrorNotification title='Unable to delete source' e={e} />,
                        type: NotificationType.Error
                    });
                }
            }
        },
        {name: 'argo-icon-warning', color: 'warning'},
        'yellow'
    );
};

export const deletePopup = async (
    ctx: ContextApis,
    resource: ResourceTreeNode,
    application: appModels.Application,
    isManaged: boolean,
    childResources: appModels.ResourceNode[],
    appChanged?: BehaviorSubject<appModels.Application>
) => {
    const deleteOptions = {
        option: 'foreground'
    };
    function handleStateChange(option: string) {
        deleteOptions.option = option;
    }

    if (resource.kind === 'Pod' && !isManaged) {
        return deletePodAction(ctx, resource, application);
    }

    return ctx.popup.prompt(
        'Delete resource',
        api => (
            <div>
                <p>
                    Are you sure you want to delete <strong>{resource.kind}</strong> <kbd>{resource.name}</kbd>?
                </p>
                <p>
                    Deleting resources can be <strong>dangerous</strong>. Be sure you understand the effects of deleting this resource before continuing. Consider asking someone to
                    review the change first.
                </p>

                {(childResources || []).length > 0 ? (
                    <React.Fragment>
                        <p>Dependent resources:</p>
                        <ul>
                            {childResources.slice(0, 4).map((child, i) => (
                                <li key={i}>
                                    <kbd>{[child.kind, child.name].join('/')}</kbd>
                                </li>
                            ))}
                            {childResources.length === 5 ? (
                                <li key='4'>
                                    <kbd>{[childResources[4].kind, childResources[4].name].join('/')}</kbd>
                                </li>
                            ) : (
                                ''
                            )}
                            {childResources.length > 5 ? <li key='N'>and {childResources.slice(4).length} more.</li> : ''}
                        </ul>
                    </React.Fragment>
                ) : (
                    ''
                )}

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
                        Background Delete {helpTip('Performs a forceful "background cascading deletion" of the resource and its dependent resources')}
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

export async function getResourceActionsMenuItems(resource: ResourceTreeNode, metadata: models.ObjectMeta, apis: ContextApis): Promise<ActionMenuItem[]> {
    return services.applications.getResourceActions(metadata.name, metadata.namespace, resource).then(actions => {
        return actions.map(action => ({
            title: action.displayName ?? action.name,
            disabled: !!action.disabled,
            iconClassName: action.iconClass,
            action: async () => {
                const confirmed = false;
                const title = action.params ? `Enter input parameters for action: ${action.name}` : `Perform ${action.name} action?`;
                await apis.popup.prompt(
                    title,
                    api => (
                        <div>
                            {!action.params && (
                                <div className='argo-form-row'>
                                    <div> Are you sure you want to perform {action.name} action?</div>
                                </div>
                            )}
                            {action.params &&
                                action.params.map((param, index) => (
                                    <div className='argo-form-row' key={index}>
                                        <FormField label={param.name} field={param.name} formApi={api} component={Text} />
                                    </div>
                                ))}
                        </div>
                    ),
                    {
                        submit: async (vals, _, close) => {
                            try {
                                const resourceActionParameters = action.params
                                    ? action.params.map(param => ({
                                          name: param.name,
                                          value: vals[param.name] || param.default,
                                          type: param.type,
                                          default: param.default
                                      }))
                                    : [];
                                await services.applications.runResourceAction(metadata.name, metadata.namespace, resource, action.name, resourceActionParameters);
                                close();
                            } catch (e) {
                                apis.notifications.show({
                                    content: <ErrorNotification title='Unable to execute resource action' e={e} />,
                                    type: NotificationType.Error
                                });
                            }
                        }
                    },
                    null,
                    null,
                    action.params
                        ? action.params.reduce((acc, res) => {
                              acc[res.name] = res.default;
                              return acc;
                          }, {} as any)
                        : {}
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
    function isTopLevelResource(res: ResourceTreeNode, app: appModels.Application): boolean {
        const uniqRes = `/${res.namespace}/${res.group}/${res.kind}/${res.name}`;
        return app.status.resources.some(resStatus => `/${resStatus.namespace}/${resStatus.group}/${resStatus.kind}/${resStatus.name}` === uniqRes);
    }

    const isPod = resource.kind === 'Pod';
    const isManaged = isTopLevelResource(resource, application);
    const childResources = findChildResources(resource, tree);

    const items: MenuItem[] = [
        ...((isManaged && [
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
                return deletePopup(apis, resource, application, isManaged, childResources, appChanged);
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

    const logsAction = services.accounts
        .canI('logs', 'get', application.spec.project + '/' + application.metadata.name)
        .then(async allowed => {
            if (allowed && (isPod || findChildPod(resource, tree))) {
                return [
                    {
                        title: 'Logs',
                        iconClassName: 'fa fa-fw fa-align-left',
                        action: () => apis.navigation.goto('.', {node: nodeKey(resource), tab: 'logs'}, {replace: true})
                    } as MenuItem
                ];
            }
            return [] as MenuItem[];
        })
        .catch(() => [] as MenuItem[]);

    if (isQuickStart) {
        return combineLatest(
            from([items]), // this resolves immediately
            concat([[] as MenuItem[]], logsAction) // this resolves at first to [] and then whatever the API returns
        ).pipe(map(res => ([] as MenuItem[]).concat(...res)));
    }

    const execAction = services.authService
        .settings()
        .then(async settings => {
            const execAllowed = settings.execEnabled && (await services.accounts.canI('exec', 'create', application.spec.project + '/' + application.metadata.name));
            if (isPod && execAllowed) {
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
                    }) as MenuItem
            );
        })
        .catch(() => [] as MenuItem[]);

    return combineLatest(
        from([items]), // this resolves immediately
        concat([[] as MenuItem[]], logsAction), // this resolves at first to [] and then whatever the API returns
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
                            tabIndex={item.disabled ? undefined : 0}
                            key={i}
                            onClick={e => {
                                e.stopPropagation();
                                if (!item.disabled) {
                                    item.action();
                                    document.body.click();
                                }
                            }}
                            onKeyDown={e => {
                                if (e.keyCode === 13 || e.key === 'Enter') {
                                    e.stopPropagation();
                                    setTimeout(() => {
                                        item.action();
                                        document.body.click();
                                    });
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

export function renderResourceActionMenu(menuItems: ActionMenuItem[]): React.ReactNode {
    return (
        <ul>
            {menuItems.map((item, i) => (
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
    );
}

export function renderResourceButtons(
    resource: ResourceTreeNode,
    application: appModels.Application,
    tree: appModels.ApplicationTree,
    apis: ContextApis,
    appChanged: BehaviorSubject<appModels.Application>
): React.ReactNode {
    const menuItems: Observable<ActionMenuItem[]> = getActionItems(resource, application, tree, apis, appChanged, true);
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
                            tooltip={item.title.toString().charAt(0).toUpperCase() + item.title.toString().slice(1)}
                        />
                    ))}
                </div>
            )}
        </DataLoader>
    );
}

export function syncStatusMessage(app: appModels.Application) {
    const source = getAppDefaultSource(app);
    const revision = getAppDefaultSyncRevision(app);
    const rev = app.status.sync.revision || (source ? source.targetRevision || 'HEAD' : 'Unknown');
    let message = source ? source?.targetRevision || 'HEAD' : 'Unknown';

    if (revision && source) {
        if (source.chart) {
            message += ' (' + revision + ')';
        } else if (revision.length >= 7 && !revision.startsWith(source.targetRevision)) {
            if (source.repoURL.startsWith('oci://')) {
                // Show "sha256: " plus the first 7 actual characters of the digest.
                if (revision.startsWith('sha256:')) {
                    message += ' (' + revision.substring(0, 14) + ')';
                } else {
                    message += ' (' + revision.substring(0, 7) + ')';
                }
            } else {
                message += ' (' + revision.substring(0, 7) + ')';
            }
        }
    }

    switch (app.status.sync.status) {
        case appModels.SyncStatuses.Synced:
            return (
                <span>
                    to{' '}
                    <Revision repoUrl={source.repoURL} revision={rev}>
                        {message}
                    </Revision>
                    {getAppDefaultSyncRevisionExtra(app)}{' '}
                </span>
            );
        case appModels.SyncStatuses.OutOfSync:
            return (
                <span>
                    from{' '}
                    <Revision repoUrl={source.repoURL} revision={rev}>
                        {message}
                    </Revision>
                    {getAppDefaultSyncRevisionExtra(app)}{' '}
                </span>
            );
        default:
            return <span>{message}</span>;
    }
}

export function hydrationStatusMessage(app: appModels.Application) {
    const drySource = app.status.sourceHydrator.currentOperation.sourceHydrator.drySource;
    const dryCommit = app.status.sourceHydrator.currentOperation.drySHA;
    const syncSource: ApplicationSource = {
        repoURL: drySource.repoURL,
        targetRevision:
            app.status.sourceHydrator.currentOperation.sourceHydrator.hydrateTo?.targetBranch || app.status.sourceHydrator.currentOperation.sourceHydrator.syncSource.targetBranch,
        path: app.status.sourceHydrator.currentOperation.sourceHydrator.syncSource.path
    };
    const hydratedCommit = app.status.sourceHydrator.currentOperation.hydratedSHA || '';

    switch (app.status.sourceHydrator.currentOperation.phase) {
        case appModels.HydrateOperationPhases.Hydrated:
            return (
                <span>
                    from{' '}
                    <Revision repoUrl={drySource.repoURL} revision={dryCommit}>
                        {drySource.targetRevision + ' (' + dryCommit.substr(0, 7) + ')'}
                    </Revision>
                    <br />
                    to{' '}
                    <Revision repoUrl={syncSource.repoURL} revision={hydratedCommit}>
                        {syncSource.targetRevision + ' (' + hydratedCommit.substr(0, 7) + ')'}
                    </Revision>
                </span>
            );
        case appModels.HydrateOperationPhases.Hydrating:
            return (
                <span>
                    from{' '}
                    <Revision repoUrl={drySource.repoURL} revision={drySource.targetRevision}>
                        {drySource.targetRevision}
                    </Revision>
                    <br />
                    to{' '}
                    <Revision repoUrl={syncSource.repoURL} revision={syncSource.targetRevision}>
                        {syncSource.targetRevision}
                    </Revision>
                </span>
            );
        case appModels.HydrateOperationPhases.Failed:
            return (
                <span>
                    from{' '}
                    <Revision repoUrl={drySource.repoURL} revision={dryCommit || drySource.targetRevision}>
                        {drySource.targetRevision}
                        {dryCommit && ' (' + dryCommit.substr(0, 7) + ')'}
                    </Revision>
                    <br />
                    to{' '}
                    <Revision repoUrl={syncSource.repoURL} revision={syncSource.targetRevision}>
                        {syncSource.targetRevision}
                    </Revision>
                </span>
            );
        default:
            return <span>{}</span>;
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
    return icon.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-health-status-title' />
    ) : (
        <i qe-id='utils-health-status-title' title={title} className={'fa ' + icon + ' utils-health-status-icon'} style={{color}} />
    );
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
    return icon.includes('fa-spin') ? (
        <SpinningIcon color={'white'} qeId='utils-health-status-title' />
    ) : (
        <i qe-id='utils-health-status-title' title={title} className={'fa ' + icon} />
    );
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
    return className.includes('fa-spin') ? <SpinningIcon color={'white'} qeId='utils-pod-phase-icon' /> : <i qe-id='utils-pod-phase-icon' className={className} />;
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
                icon = 'fa-trash';
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
        return className.includes('fa-spin') ? <SpinningIcon color={color} qeId='utils-resource-result-icon' /> : <i title={title} className={className} style={{color}} />;
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

export const OperationState = ({app, quiet, isButton}: {app: appModels.Application; quiet?: boolean; isButton?: boolean}) => {
    const appOperationState = getAppOperationState(app);
    if (appOperationState === undefined) {
        return <React.Fragment />;
    }
    if (quiet && [appModels.OperationPhases.Running, appModels.OperationPhases.Failed, appModels.OperationPhases.Error].indexOf(appOperationState.phase) === -1) {
        return <React.Fragment />;
    }

    return (
        <React.Fragment>
            <OperationPhaseIcon app={app} isButton={isButton} /> {getOperationStateTitle(app)}
        </React.Fragment>
    );
};

function isPodInitializedConditionTrue(status: any): boolean {
    if (!status?.conditions) {
        return false;
    }

    for (const condition of status.conditions) {
        if (condition.type !== 'Initialized') {
            continue;
        }
        return condition.status === 'True';
    }

    return false;
}

// isPodPhaseTerminal returns true if the pod's phase is terminal.
function isPodPhaseTerminal(phase: appModels.PodPhase): boolean {
    return phase === appModels.PodPhase.PodFailed || phase === appModels.PodPhase.PodSucceeded;
}

export function getPodStateReason(pod: appModels.State): {message: string; reason: string; netContainerStatuses: any[]} {
    if (!pod.status) {
        return {reason: 'Unknown', message: '', netContainerStatuses: []};
    }

    const podPhase = pod.status.phase;
    let reason = podPhase;
    let message = '';
    if (pod.status.reason) {
        reason = pod.status.reason;
    }

    let netContainerStatuses = pod.status.initContainerStatuses || [];
    netContainerStatuses = netContainerStatuses.concat(pod.status.containerStatuses || []);

    for (const condition of pod.status.conditions || []) {
        if (condition.type === 'PodScheduled' && condition.reason === 'SchedulingGated') {
            reason = 'SchedulingGated';
        }
    }

    const initContainers: Record<string, any> = {};

    for (const container of pod.spec.initContainers ?? []) {
        initContainers[container.name] = container;
    }

    let initializing = false;
    const initContainerStatuses = pod.status.initContainerStatuses || [];
    for (let i = 0; i < initContainerStatuses.length; i++) {
        const container = initContainerStatuses[i];
        if (container.state.terminated && container.state.terminated.exitCode === 0) {
            continue;
        }

        if (container.started && initContainers[container.name].restartPolicy === 'Always') {
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
            reason = `Init:${i}/${(pod.spec.initContainers || []).length}`;
        }
        initializing = true;
        break;
    }

    if (!initializing || isPodInitializedConditionTrue(pod.status)) {
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
    } else if ((pod as any).metadata.deletionTimestamp && !isPodPhaseTerminal(podPhase)) {
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
        // e.g. if there are multiple conditions set with same name then the one which comes first is evaluated
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
    if (source?.kustomize?.images) {
        return source.kustomize.images.length;
    }
    if (source?.helm?.parameters) {
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
    return getAppSpecDefaultSource(app.spec);
}

// getAppDefaultSyncRevision gets the first app revisions from `status.sync.revisions` or, if that list is missing or empty, the `revision`
// field.
export function getAppDefaultSyncRevision(app?: appModels.Application) {
    if (!app || !app.status || !app.status.sync) {
        return '';
    }
    return app.status.sync.revisions && app.status.sync.revisions.length > 0 ? app.status.sync.revisions[0] : app.status.sync.revision;
}

// getAppDefaultOperationSyncRevision gets the first app revisions from `status.operationState.syncResult.revisions` or, if that list is missing or empty, the `revision`
// field.
export function getAppDefaultOperationSyncRevision(app?: appModels.Application) {
    if (!app || !app.status || !app.status.operationState || !app.status.operationState.syncResult) {
        return '';
    }
    return app.status.operationState.syncResult.revisions && app.status.operationState.syncResult.revisions.length > 0
        ? app.status.operationState.syncResult.revisions[0]
        : app.status.operationState.syncResult.revision;
}

// getAppCurrentVersion gets the first app revisions from `status.sync.revisions` or, if that list is missing or empty, the `revision`
// field.
export function getAppCurrentVersion(app?: appModels.Application): number | null {
    if (!app || !app.status || !app.status.history || app.status.history.length === 0) {
        return null;
    }
    return app.status.history[app.status.history.length - 1].id;
}

// getAppDefaultSyncRevisionExtra gets the extra message with others revision count
export function getAppDefaultSyncRevisionExtra(app?: appModels.Application) {
    if (!app || !app.status || !app.status.sync) {
        return '';
    }

    if (app.status.sync.revisions && app.status.sync.revisions.length > 0) {
        return ` and (${app.status.sync.revisions.length - 1}) more`;
    }

    return '';
}

// getAppDefaultOperationSyncRevisionExtra gets the first app revisions from `status.operationState.syncResult.revisions` or, if that list is missing or empty, the `revision`
// field.
export function getAppDefaultOperationSyncRevisionExtra(app?: appModels.Application) {
    if (!app || !app.status || !app.status.operationState || !app.status.operationState.syncResult || !app.status.operationState.syncResult.revisions) {
        return '';
    }

    if (app.status.operationState.syncResult.revisions.length > 0) {
        return ` and (${app.status.operationState.syncResult.revisions.length - 1}) more`;
    }
    return '';
}

export function getAppSpecDefaultSource(spec: appModels.ApplicationSpec) {
    if (spec.sourceHydrator) {
        return {
            repoURL: spec.sourceHydrator.drySource.repoURL,
            targetRevision: spec.sourceHydrator.syncSource.targetBranch,
            path: spec.sourceHydrator.syncSource.path
        };
    }
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
    const createdAt = moment.utc(creationTimestamp).local().format('MM/DD/YYYY HH:mm:ss');
    const fromNow = moment.utc(creationTimestamp).local().fromNow();
    return (
        <span>
            {createdAt}
            <i style={{padding: '2px'}} /> ({fromNow})
        </span>
    );
}

/*
 * formatStatefulSetChange reformats a single line describing changes to immutable fields in a StatefulSet.
 * It extracts the field name and its "from" and "to" values for better readability.
 */
function formatStatefulSetChange(line: string): string {
    if (line.startsWith('-')) {
        // Remove leading "- " from the line and split into field and changes
        const [field, changes] = line.substring(2).split(':');
        if (changes) {
            // Split "from: X to: Y" into separate lines with aligned values
            const [from, to] = changes.split('to:').map(s => s.trim());
            return `   - ${field}:\n      from: ${from.replace('from:', '').trim()}\n      to:   ${to}`;
        }
    }
    return line;
}

export function formatOperationMessage(message: string): string {
    if (!message) {
        return message;
    }

    // Format immutable fields error message
    if (message.includes('attempting to change immutable fields:')) {
        const [header, ...details] = message.split('\n');
        const formattedDetails = details
            // Remove empty lines
            .filter(line => line.trim())
            // Use helper function
            .map(formatStatefulSetChange)
            .join('\n');

        return `${header}\n${formattedDetails}`;
    }

    return message;
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

export function getAppUrl(app: appModels.Application): string {
    if (typeof app.metadata.namespace === 'undefined') {
        return `applications/${app.metadata.name}`;
    }
    return `applications/${app.metadata.namespace}/${app.metadata.name}`;
}

export const getProgressiveSyncStatusIcon = ({status, isButton}: {status: string; isButton?: boolean}) => {
    const getIconProps = () => {
        switch (status) {
            case 'Healthy':
                return {icon: 'fa-check-circle', color: COLORS.health.healthy};
            case 'Progressing':
                return {icon: 'fa-circle-notch fa-spin', color: COLORS.health.progressing};
            case 'Pending':
                return {icon: 'fa-clock', color: COLORS.health.degraded};
            case 'Waiting':
                return {icon: 'fa-clock', color: COLORS.sync.out_of_sync};
            case 'Error':
                return {icon: 'fa-times-circle', color: COLORS.health.degraded};
            default:
                return {icon: 'fa-question-circle', color: COLORS.sync.unknown};
        }
    };

    const {icon, color} = getIconProps();
    const className = `fa ${icon}${isButton ? ' application-status-panel__item-value__status-button' : ''}`;
    return <i className={className} style={{color}} />;
};

export const getProgressiveSyncStatusColor = (status: string): string => {
    switch (status) {
        case 'Waiting':
            return COLORS.sync.out_of_sync;
        case 'Pending':
            return COLORS.health.degraded;
        case 'Progressing':
            return COLORS.health.progressing;
        case 'Healthy':
            return COLORS.health.healthy;
        case 'Error':
            return COLORS.health.degraded;
        default:
            return COLORS.sync.unknown;
    }
};
