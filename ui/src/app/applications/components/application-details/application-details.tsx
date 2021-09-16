import {Checkbox as ArgoCheckbox, DropDownMenu, NotificationType, SlidingPanel} from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {RouteComponentProps} from 'react-router';
import {BehaviorSubject, combineLatest, from, merge, Observable} from 'rxjs';
import {delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';

import {DataLoader, EmptyState, ErrorNotification, ObservableQuery, Page, Paginate, Revision, Timestamp} from '../../../shared/components';
import {AppContext, ContextApis} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {AppDetailsPreferences, AppsDetailsViewType, services} from '../../../shared/services';

import {ApplicationConditions} from '../application-conditions/application-conditions';
import {ApplicationDeploymentHistory} from '../application-deployment-history/application-deployment-history';
import {ApplicationOperationState} from '../application-operation-state/application-operation-state';
import {PodView} from '../application-pod-view/pod-view';
import {ApplicationResourceTree, ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ResourceDetails} from '../resource-details/resource-details';
import * as AppUtils from '../utils';
import {ApplicationResourceList} from './application-resource-list';
import {Filters} from './application-resource-filter';

require('./application-details.scss');

interface ApplicationDetailsState {
    page: number;
    revision?: string;
}

interface FilterInput {
    name: string[];
    kind: string[];
    health: string[];
    sync: string[];
    namespace: string[];
}

export const NodeInfo = (node?: string): {key: string; container: number} => {
    const nodeContainer = {key: '', container: 0};
    if (node) {
        const parts = node.split('/');
        nodeContainer.key = parts.slice(0, 4).join('/');
        nodeContainer.container = parseInt(parts[4] || '0', 10);
    }
    return nodeContainer;
};

export const SelectNode = (fullName: string, containerIndex = 0, tab: string = null, appContext: ContextApis) => {
    const node = fullName ? `${fullName}/${containerIndex}` : null;
    appContext.navigation.goto('.', {node, tab});
};

export class ApplicationDetails extends React.Component<RouteComponentProps<{name: string}>, ApplicationDetailsState> {
    public static contextTypes = {
        apis: PropTypes.object
    };

    private appChanged = new BehaviorSubject<appModels.Application>(null);

    constructor(props: RouteComponentProps<{name: string}>) {
        super(props);
        this.state = {page: 0};
    }

    private get showOperationState() {
        return new URLSearchParams(this.props.history.location.search).get('operation') === 'true';
    }

    private get showConditions() {
        return new URLSearchParams(this.props.history.location.search).get('conditions') === 'true';
    }

    private get selectedRollbackDeploymentIndex() {
        return parseInt(new URLSearchParams(this.props.history.location.search).get('rollback'), 10);
    }

    private get selectedNodeInfo() {
        return NodeInfo(new URLSearchParams(this.props.history.location.search).get('node'));
    }

    private get selectedNodeKey() {
        const nodeContainer = this.selectedNodeInfo;
        return nodeContainer.key;
    }

    public render() {
        return (
            <ObservableQuery>
                {q => (
                    <DataLoader
                        errorRenderer={error => <Page title='Application Details'>{error}</Page>}
                        loadingRenderer={() => <Page title='Application Details'>Loading...</Page>}
                        input={this.props.match.params.name}
                        load={name =>
                            combineLatest([this.loadAppInfo(name), services.viewPreferences.getPreferences(), q]).pipe(
                                map(items => {
                                    const pref = items[1].appDetails;
                                    const params = items[2];
                                    if (params.get('resource') != null) {
                                        pref.resourceFilter = params
                                            .get('resource')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('view') != null) {
                                        pref.view = params.get('view') as AppsDetailsViewType;
                                    }
                                    if (params.get('orphaned') != null) {
                                        pref.orphanedResources = params.get('orphaned') === 'true';
                                    }
                                    return {...items[0], pref};
                                })
                            )
                        }>
                        {({application, tree, pref}: {application: appModels.Application; tree: appModels.ApplicationTree; pref: AppDetailsPreferences}) => {
                            tree.nodes = tree.nodes || [];
                            const treeFilter = this.getTreeFilter(pref.resourceFilter);
                            const setFilter = (items: string[]) => {
                                this.appContext.apis.navigation.goto('.', {resource: items.join(',')});
                                services.viewPreferences.updatePreferences({appDetails: {...pref, resourceFilter: items}});
                            };
                            const clearFilter = () => setFilter([]);
                            const refreshing = application.metadata.annotations && application.metadata.annotations[appModels.AnnotationRefreshKey];
                            const appNodesByName = this.groupAppNodesByKey(application, tree);
                            const selectedItem = (this.selectedNodeKey && appNodesByName.get(this.selectedNodeKey)) || null;
                            const isAppSelected = selectedItem === application;
                            const selectedNode = !isAppSelected && (selectedItem as appModels.ResourceNode);
                            const operationState = application.status.operationState;
                            const conditions = application.status.conditions || [];
                            const syncResourceKey = new URLSearchParams(this.props.history.location.search).get('deploy');
                            const tab = new URLSearchParams(this.props.history.location.search).get('tab');

                            const orphaned: appModels.ResourceStatus[] = pref.orphanedResources
                                ? (tree.orphanedNodes || []).map(node => ({
                                      ...node,
                                      status: null,
                                      health: null
                                  }))
                                : [];
                            const filteredRes = application.status.resources.concat(orphaned).filter(res => {
                                const resNode: ResourceTreeNode = {...res, root: null, info: null, parentRefs: [], resourceVersion: '', uid: ''};
                                resNode.root = resNode;
                                return this.filterTreeNode(resNode, treeFilter);
                            });

                            return (
                                <div className='application-details'>
                                    <Page
                                        title='Application Details'
                                        toolbar={{
                                            breadcrumbs: [{title: 'Applications', path: '/applications'}, {title: this.props.match.params.name}],
                                            actionMenu: {items: this.getApplicationActionMenu(application, true)},
                                            tools: (
                                                <React.Fragment key='app-list-tools'>
                                                    <div className='application-details__view-type'>
                                                        <i
                                                            className={classNames('fa fa-sitemap', {selected: pref.view === 'tree'})}
                                                            title='Tree'
                                                            onClick={() => {
                                                                this.appContext.apis.navigation.goto('.', {view: 'tree'});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: 'tree'}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-th', {selected: pref.view === 'pods'})}
                                                            title='Pods'
                                                            onClick={() => {
                                                                this.appContext.apis.navigation.goto('.', {view: 'pods'});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: 'pods'}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-network-wired', {selected: pref.view === 'network'})}
                                                            title='Network'
                                                            onClick={() => {
                                                                this.appContext.apis.navigation.goto('.', {view: 'network'});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: 'network'}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-th-list', {selected: pref.view === 'list'})}
                                                            title='List'
                                                            onClick={() => {
                                                                this.appContext.apis.navigation.goto('.', {view: 'list'});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: 'list'}});
                                                            }}
                                                        />
                                                    </div>
                                                </React.Fragment>
                                            )
                                        }}>
                                        <div className='application-details__status-panel'>
                                            <ApplicationStatusPanel
                                                application={application}
                                                showOperation={() => this.setOperationStatusVisible(true)}
                                                showConditions={() => this.setConditionsStatusVisible(true)}
                                                showMetadataInfo={revision => this.setState({...this.state, revision})}
                                            />
                                        </div>
                                        <div className='application-details__tree'>
                                            {refreshing && <p className='application-details__refreshing-label'>Refreshing</p>}

                                            {(tree.orphanedNodes || []).length > 0 && (
                                                <div className='application-details__orphaned-filter'>
                                                    <ArgoCheckbox
                                                        checked={!!pref.orphanedResources}
                                                        id='orphanedFilter'
                                                        onChange={val => {
                                                            this.appContext.apis.navigation.goto('.', {orphaned: val});
                                                            services.viewPreferences.updatePreferences({appDetails: {...pref, orphanedResources: val}});
                                                        }}
                                                    />{' '}
                                                    <label htmlFor='orphanedFilter'>SHOW ORPHANED</label>
                                                </div>
                                            )}
                                            {((pref.view === 'tree' || pref.view === 'network') && (
                                                <Filters pref={pref} tree={tree} onSetFilter={setFilter} onClearFilter={clearFilter}>
                                                    <ApplicationResourceTree
                                                        nodeFilter={node => this.filterTreeNode(node, treeFilter)}
                                                        selectedNodeFullName={this.selectedNodeKey}
                                                        onNodeClick={fullName => this.selectNode(fullName)}
                                                        nodeMenu={node =>
                                                            AppUtils.renderResourceMenu(node, application, tree, this.appContext, this.appChanged, () =>
                                                                this.getApplicationActionMenu(application, false)
                                                            )
                                                        }
                                                        tree={tree}
                                                        app={application}
                                                        showOrphanedResources={pref.orphanedResources}
                                                        useNetworkingHierarchy={pref.view === 'network'}
                                                        onClearFilter={clearFilter}
                                                    />
                                                </Filters>
                                            )) ||
                                                (pref.view === 'pods' && (
                                                    <PodView
                                                        tree={tree}
                                                        app={application}
                                                        onItemClick={fullName => this.selectNode(fullName)}
                                                        nodeMenu={node =>
                                                            AppUtils.renderResourceMenu(node, application, tree, this.appContext, this.appChanged, () =>
                                                                this.getApplicationActionMenu(application, false)
                                                            )
                                                        }
                                                    />
                                                )) || (
                                                    <div>
                                                        <Filters pref={pref} tree={tree} onSetFilter={setFilter} onClearFilter={clearFilter}>
                                                            {(filteredRes.length > 0 && (
                                                                <Paginate
                                                                    page={this.state.page}
                                                                    data={filteredRes}
                                                                    onPageChange={page => this.setState({page})}
                                                                    preferencesKey='application-details'>
                                                                    {data => (
                                                                        <ApplicationResourceList
                                                                            onNodeClick={fullName => this.selectNode(fullName)}
                                                                            resources={data}
                                                                            nodeMenu={node =>
                                                                                AppUtils.renderResourceMenu(
                                                                                    {...node, root: node},
                                                                                    application,
                                                                                    tree,
                                                                                    this.appContext,
                                                                                    this.appChanged,
                                                                                    () => this.getApplicationActionMenu(application, false)
                                                                                )
                                                                            }
                                                                        />
                                                                    )}
                                                                </Paginate>
                                                            )) || (
                                                                <EmptyState icon='fa fa-search'>
                                                                    <h4>No resources found</h4>
                                                                    <h5>Try to change filter criteria</h5>
                                                                </EmptyState>
                                                            )}
                                                        </Filters>
                                                    </div>
                                                )}
                                        </div>
                                        <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => this.selectNode('')}>
                                            <ResourceDetails
                                                tree={tree}
                                                application={application}
                                                isAppSelected={isAppSelected}
                                                updateApp={app => this.updateApp(app)}
                                                selectedNode={selectedNode}
                                                tab={tab}
                                            />
                                        </SlidingPanel>
                                        <ApplicationSyncPanel
                                            application={application}
                                            hide={() => AppUtils.showDeploy(null, this.appContext)}
                                            selectedResource={syncResourceKey}
                                        />
                                        <SlidingPanel isShown={this.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                                            {this.selectedRollbackDeploymentIndex > -1 && (
                                                <ApplicationDeploymentHistory
                                                    app={application}
                                                    selectedRollbackDeploymentIndex={this.selectedRollbackDeploymentIndex}
                                                    rollbackApp={info => this.rollbackApplication(info, application)}
                                                    selectDeployment={i => this.setRollbackPanelVisible(i)}
                                                />
                                            )}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={this.showOperationState && !!operationState} onClose={() => this.setOperationStatusVisible(false)}>
                                            {operationState && <ApplicationOperationState application={application} operationState={operationState} />}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={this.showConditions && !!conditions} onClose={() => this.setConditionsStatusVisible(false)}>
                                            {conditions && <ApplicationConditions conditions={conditions} />}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={!!this.state.revision} isMiddle={true} onClose={() => this.setState({revision: null})}>
                                            {this.state.revision && (
                                                <DataLoader load={() => services.applications.revisionMetadata(application.metadata.name, this.state.revision)}>
                                                    {metadata => (
                                                        <div className='white-box' style={{marginTop: '1.5em'}}>
                                                            <div className='white-box__details'>
                                                                <div className='row white-box__details-row'>
                                                                    <div className='columns small-3'>SHA:</div>
                                                                    <div className='columns small-9'>
                                                                        <Revision repoUrl={application.spec.source.repoURL} revision={this.state.revision} />
                                                                    </div>
                                                                </div>
                                                            </div>
                                                            <div className='white-box__details'>
                                                                <div className='row white-box__details-row'>
                                                                    <div className='columns small-3'>Date:</div>
                                                                    <div className='columns small-9'>
                                                                        <Timestamp date={metadata.date} />
                                                                    </div>
                                                                </div>
                                                            </div>
                                                            <div className='white-box__details'>
                                                                <div className='row white-box__details-row'>
                                                                    <div className='columns small-3'>Tags:</div>
                                                                    <div className='columns small-9'>
                                                                        {((metadata.tags || []).length > 0 && metadata.tags.join(', ')) || 'No tags'}
                                                                    </div>
                                                                </div>
                                                            </div>
                                                            <div className='white-box__details'>
                                                                <div className='row white-box__details-row'>
                                                                    <div className='columns small-3'>Author:</div>
                                                                    <div className='columns small-9'>{metadata.author}</div>
                                                                </div>
                                                            </div>
                                                            <div className='white-box__details'>
                                                                <div className='row white-box__details-row'>
                                                                    <div className='columns small-3'>Message:</div>
                                                                    <div className='columns small-9' style={{display: 'flex', alignItems: 'center'}}>
                                                                        <div className='application-details__commit-message'>{metadata.message}</div>
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    )}
                                                </DataLoader>
                                            )}
                                        </SlidingPanel>
                                    </Page>
                                </div>
                            );
                        }}
                    </DataLoader>
                )}
            </ObservableQuery>
        );
    }

    private getApplicationActionMenu(app: appModels.Application, needOverlapLabelOnNarrowScreen: boolean) {
        const refreshing = app.metadata.annotations && app.metadata.annotations[appModels.AnnotationRefreshKey];
        const fullName = AppUtils.nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
        const ActionMenuItem = (prop: {actionLabel: string}) => <span className={needOverlapLabelOnNarrowScreen ? 'show-for-large' : ''}>{prop.actionLabel}</span>;
        return [
            {
                iconClassName: 'fa fa-info-circle',
                title: <ActionMenuItem actionLabel='App Details' />,
                action: () => this.selectNode(fullName)
            },
            {
                iconClassName: 'fa fa-file-medical',
                title: <ActionMenuItem actionLabel='App Diff' />,
                action: () => this.selectNode(fullName, 0, 'diff'),
                disabled: app.status.sync.status === appModels.SyncStatuses.Synced
            },
            {
                iconClassName: 'fa fa-sync',
                title: <ActionMenuItem actionLabel='Sync' />,
                action: () => AppUtils.showDeploy('all', this.appContext)
            },
            {
                iconClassName: 'fa fa-info-circle',
                title: <ActionMenuItem actionLabel='Sync Status' />,
                action: () => this.setOperationStatusVisible(true),
                disabled: !app.status.operationState
            },
            {
                iconClassName: 'fa fa-history',
                title: <ActionMenuItem actionLabel='History and rollback' />,
                action: () => this.setRollbackPanelVisible(0),
                disabled: !app.status.operationState
            },
            {
                iconClassName: 'fa fa-times-circle',
                title: <ActionMenuItem actionLabel='Delete' />,
                action: () => this.deleteApplication()
            },
            {
                iconClassName: classNames('fa fa-redo', {'status-icon--spin': !!refreshing}),
                title: (
                    <React.Fragment>
                        <ActionMenuItem actionLabel='Refresh' />{' '}
                        <DropDownMenu
                            items={[
                                {
                                    title: 'Hard Refresh',
                                    action: () => !refreshing && services.applications.get(app.metadata.name, 'hard')
                                }
                            ]}
                            anchor={() => <i className='fa fa-caret-down' />}
                        />
                    </React.Fragment>
                ),
                disabled: !!refreshing,
                action: () => {
                    if (!refreshing) {
                        services.applications.get(app.metadata.name, 'normal');
                        AppUtils.setAppRefreshing(app);
                        this.appChanged.next(app);
                    }
                }
            }
        ];
    }

    private filterTreeNode(node: ResourceTreeNode, filterInput: FilterInput): boolean {
        const syncStatuses = filterInput.sync.map(item => (item === 'OutOfSync' ? ['OutOfSync', 'Unknown'] : [item])).reduce((first, second) => first.concat(second), []);

        const root = node.root || ({} as ResourceTreeNode);
        const hook = root && root.hook;
        if (
            (filterInput.name.length === 0 || filterInput.name.indexOf(node.name) > -1) &&
            (filterInput.kind.length === 0 || filterInput.kind.indexOf(node.kind) > -1) &&
            // include if node's root sync matches filter
            (syncStatuses.length === 0 || hook || (root.status && syncStatuses.indexOf(root.status) > -1)) &&
            // include if node or node's root health matches filter
            (filterInput.health.length === 0 ||
                hook ||
                (root.health && filterInput.health.indexOf(root.health.status) > -1) ||
                (node.health && filterInput.health.indexOf(node.health.status) > -1)) &&
            (filterInput.namespace.length === 0 || filterInput.namespace.includes(node.namespace))
        ) {
            return true;
        }

        return false;
    }

    private loadAppInfo(name: string): Observable<{application: appModels.Application; tree: appModels.ApplicationTree}> {
        return from(services.applications.get(name))
            .pipe(
                mergeMap(app => {
                    const fallbackTree = {
                        nodes: app.status.resources.map(res => ({...res, parentRefs: [], info: [], resourceVersion: '', uid: ''})),
                        orphanedNodes: [],
                        hosts: []
                    } as appModels.ApplicationTree;
                    return combineLatest(
                        merge(
                            from([app]),
                            this.appChanged.pipe(filter(item => !!item)),
                            AppUtils.handlePageVisibility(() =>
                                services.applications
                                    .watch({name})
                                    .pipe(
                                        map(watchEvent => {
                                            if (watchEvent.type === 'DELETED') {
                                                this.onAppDeleted();
                                            }
                                            return watchEvent.application;
                                        })
                                    )
                                    .pipe(repeat())
                                    .pipe(retryWhen(errors => errors.pipe(delay(500))))
                            )
                        ),
                        merge(
                            from([fallbackTree]),
                            services.applications.resourceTree(name).catch(() => fallbackTree),
                            AppUtils.handlePageVisibility(() =>
                                services.applications
                                    .watchResourceTree(name)
                                    .pipe(repeat())
                                    .pipe(retryWhen(errors => errors.pipe(delay(500))))
                            )
                        )
                    );
                })
            )
            .pipe(filter(([application, tree]) => !!application && !!tree))
            .pipe(map(([application, tree]) => ({application, tree})));
    }

    private onAppDeleted() {
        this.appContext.apis.notifications.show({type: NotificationType.Success, content: `Application '${this.props.match.params.name}' was deleted`});
        this.appContext.apis.navigation.goto('/applications');
    }

    private async updateApp(app: appModels.Application) {
        const latestApp = await services.applications.get(app.metadata.name);
        latestApp.metadata.labels = app.metadata.labels;
        latestApp.metadata.annotations = app.metadata.annotations;
        latestApp.spec = app.spec;
        const updatedApp = await services.applications.update(latestApp);
        this.appChanged.next(updatedApp);
    }

    private groupAppNodesByKey(application: appModels.Application, tree: appModels.ApplicationTree) {
        const nodeByKey = new Map<string, appModels.ResourceDiff | appModels.ResourceNode | appModels.Application>();
        tree.nodes.concat(tree.orphanedNodes || []).forEach(node => nodeByKey.set(AppUtils.nodeKey(node), node));
        nodeByKey.set(AppUtils.nodeKey({group: 'argoproj.io', kind: application.kind, name: application.metadata.name, namespace: application.metadata.namespace}), application);
        return nodeByKey;
    }

    private getTreeFilter(filterInput: string[]): FilterInput {
        const name = new Array<string>();
        const kind = new Array<string>();
        const health = new Array<string>();
        const sync = new Array<string>();
        const namespace = new Array<string>();
        for (const item of filterInput || []) {
            const [type, val] = item.split(':');
            switch (type) {
                case 'name':
                    name.push(val);
                    break;
                case 'kind':
                    kind.push(val);
                    break;
                case 'health':
                    health.push(val);
                    break;
                case 'sync':
                    sync.push(val);
                    break;
                case 'namespace':
                    namespace.push(val);
                    break;
            }
        }
        return {kind, health, sync, namespace, name};
    }

    private setOperationStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', {operation: isVisible});
    }

    private setConditionsStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', {conditions: isVisible});
    }

    private setRollbackPanelVisible(selectedDeploymentIndex = 0) {
        this.appContext.apis.navigation.goto('.', {rollback: selectedDeploymentIndex});
    }

    private selectNode(fullName: string, containerIndex = 0, tab: string = null) {
        SelectNode(fullName, containerIndex, tab, this.appContext.apis);
    }

    private async rollbackApplication(revisionHistory: appModels.RevisionHistory, application: appModels.Application) {
        try {
            const needDisableRollback = application.spec.syncPolicy && application.spec.syncPolicy.automated;
            let confirmationMessage = `Are you sure you want to rollback application '${this.props.match.params.name}'?`;
            if (needDisableRollback) {
                confirmationMessage = `Auto-Sync needs to be disabled in order for rollback to occur.
Are you sure you want to disable auto-sync and rollback application '${this.props.match.params.name}'?`;
            }

            const confirmed = await this.appContext.apis.popup.confirm('Rollback application', confirmationMessage);
            if (confirmed) {
                if (needDisableRollback) {
                    const update = JSON.parse(JSON.stringify(application)) as appModels.Application;
                    update.spec.syncPolicy = {automated: null};
                    await services.applications.update(update);
                }
                await services.applications.rollback(this.props.match.params.name, revisionHistory.id);
                this.appChanged.next(await services.applications.get(this.props.match.params.name));
                this.setRollbackPanelVisible(-1);
            }
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to rollback application' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private async deleteApplication() {
        await AppUtils.deleteApplication(this.props.match.params.name, this.appContext.apis);
    }
}
