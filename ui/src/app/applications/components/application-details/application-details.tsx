import {DropDownMenu, MenuItem, NotificationType, SlidingPanel, Tab, Tabs, TopBarFilter} from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Checkbox} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {BehaviorSubject, Observable} from 'rxjs';
import {
    DataLoader,
    EmptyState,
    ErrorNotification,
    EventsList,
    ObservableQuery,
    Page,
    Paginate,
    YamlEditor,
} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {AppDetailsPreferences, AppsDetailsViewType, services} from '../../../shared/services';

import {SyncStatuses} from '../../../shared/models';
import {ApplicationConditions} from '../application-conditions/application-conditions';
import {ApplicationDeploymentHistory} from '../application-deployment-history/application-deployment-history';
import {ApplicationNodeInfo} from '../application-node-info/application-node-info';
import {ApplicationOperationState} from '../application-operation-state/application-operation-state';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {ApplicationResourceEvents} from '../application-resource-events/application-resource-events';
import {ApplicationResourceTree, ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {ApplicationSummary} from '../application-summary/application-summary';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {PodsLogsViewer} from '../pod-logs-viewer/pod-logs-viewer';
import * as AppUtils from '../utils';
import {isSameNode, nodeKey} from '../utils';
import {ApplicationResourceList} from './application-resource-list';

const jsonMergePatch = require('json-merge-patch');

require('./application-details.scss');

export class ApplicationDetails extends React.Component<RouteComponentProps<{ name: string; }>, {page: number}> {

    public static contextTypes = {
        apis: PropTypes.object,
    };

    private refreshRequested = new BehaviorSubject(null);

    constructor(props: RouteComponentProps<{ name: string; }>) {
        super(props);
        this.state = { page: 0 };
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
        const nodeContainer = { key: '', container: 0 };
        const node = new URLSearchParams(this.props.location.search).get('node');
        if (node) {
            const parts = node.split('/');
            nodeContainer.key = parts.slice(0, 4).join('/');
            nodeContainer.container = parseInt(parts[4] || '0', 10);
        }
        return nodeContainer;
    }

    private get selectedNodeKey() {
        const nodeContainer = this.selectedNodeInfo;
        return nodeContainer.key;
    }

    public render() {
        return (
            <ObservableQuery>
            {(q) => (
            <DataLoader
                errorRenderer={(error) => <Page title='Application Details'>{error}</Page>}
                loadingRenderer={() => <Page title='Application Details'>Loading...</Page>}
                input={this.props.match.params.name}
                load={(name) => Observable.combineLatest(this.loadAppInfo(name), services.viewPreferences.getPreferences(), q).map((items) => {
                    const pref = items[1].appDetails;
                    const params = items[2];
                    if (params.get('resource') != null) {
                        pref.resourceFilter = params.get('resource').split(',').filter((item) => !!item);
                    }
                    if (params.get('view') != null) {
                        pref.view = params.get('view') as AppsDetailsViewType;
                    }
                    return {...items[0], pref};
                })}>

                {({application, tree, pref}: {application: appModels.Application, tree: appModels.ApplicationTree, pref: AppDetailsPreferences}) => {
                    tree.nodes = tree.nodes || [];
                    const kindsSet = new Set<string>(tree.nodes.map((item) => item.kind));
                    const treeFilter = this.getTreeFilter(pref.resourceFilter);
                    treeFilter.kind.forEach((kind) => { kindsSet.add(kind); });
                    const kinds = Array.from(kindsSet);
                    const noKindsFilter = pref.resourceFilter.filter((item) => item.indexOf('kind:') !== 0);
                    const refreshing = application.metadata.annotations && application.metadata.annotations[appModels.AnnotationRefreshKey];

                    const filter: TopBarFilter<string> = {
                        items: [
                            { content: () => <span>Sync</span> },
                            { value: 'sync:Synced', label: 'Synced' },
                            // Unhealthy includes 'Unknown' and 'OutOfSync'
                            { value: 'sync:OutOfSync', label: 'OutOfSync' },
                            { content: () => <span>Health</span> },
                            { value: 'health:Healthy', label: 'Healthy' },
                            // Unhealthy includes 'Unknown', 'Progressing', 'Degraded' and 'Missing'
                            { value: 'health:Unhealthy', label: 'Unhealthy' },
                            { content: (setSelection) => (
                                <div>
                                    Kinds <a onClick={() => setSelection(noKindsFilter.concat(kinds.map((kind) => `kind:${kind}`)))}>all</a> / <a
                                        onClick={() => setSelection(noKindsFilter)}>none</a>
                                </div>
                            ) },
                            ...kinds.sort().map((kind) => ({ value: `kind:${kind}`, label: kind })),
                        ],
                        selectedValues: pref.resourceFilter,
                        selectionChanged: (items) => {
                            this.appContext.apis.navigation.goto('.', { resource: `${items.join(',')}`});
                            services.viewPreferences.updatePreferences({ appDetails: { ...pref, resourceFilter: items } });
                        },
                    };

                    const appNodesByName = this.groupAppNodesByKey(application, tree);
                    const selectedItem = this.selectedNodeKey && appNodesByName.get(this.selectedNodeKey) || null;
                    const isAppSelected = selectedItem === application;
                    const selectedNode = !isAppSelected && selectedItem as appModels.ResourceNode;
                    const operationState = application.status.operationState;
                    const conditions = application.status.conditions || [];
                    const syncResourceKey = new URLSearchParams(this.props.history.location.search).get('deploy');
                    const tab = new URLSearchParams(this.props.history.location.search).get('tab');
                    const filteredRes = application.status.resources.filter((res) => {
                        const resNode: ResourceTreeNode = {...res, root: null, info: null, parentRefs: [], resourceVersion: '', uid: ''};
                        resNode.root = resNode;
                        return this.filterTreeNode(resNode, treeFilter);
                    });
                    return (
                        <div className='application-details'>
                        <Page
                            title='Application Details'
                            toolbar={{
                                filter,
                                breadcrumbs: [{title: 'Applications', path: '/applications' }, { title: this.props.match.params.name }],
                                actionMenu: {items: this.getApplicationActionMenu(application)},
                                tools: (
                                    <React.Fragment key='app-list-tools'>
                                        <div className='application-details__view-type'>
                                            <i className={classNames('fa fa-sitemap', {selected: pref.view === 'tree'})} onClick={() => {
                                                this.appContext.apis.navigation.goto('.', { view: 'tree' });
                                                services.viewPreferences.updatePreferences({ appDetails: {...pref, view: 'tree'} });
                                            }} />
                                            <i className={classNames('fa fa-network-wired', {selected: pref.view === 'network'})} onClick={() => {
                                                this.appContext.apis.navigation.goto('.', { view: 'network' });
                                                services.viewPreferences.updatePreferences({ appDetails: {...pref, view: 'network'} });
                                            }} />
                                            <i className={classNames('fa fa-th-list', {selected: pref.view === 'list'})} onClick={() => {
                                                this.appContext.apis.navigation.goto('.', { view: 'list' });
                                                services.viewPreferences.updatePreferences({ appDetails: {...pref, view: 'list'} });
                                            }} />
                                        </div>
                                    </React.Fragment>
                                ),
                            }}>
                            <div className='application-details__status-panel'>
                                <ApplicationStatusPanel application={application}
                                    showOperation={() => this.setOperationStatusVisible(true)}
                                    showConditions={() => this.setConditionsStatusVisible(true)}
                                />
                            </div>
                            <div className='application-details__tree'>
                                {refreshing && <p className='application-details__refreshing-label'>Refreshing</p>}
                                {(pref.view === 'tree' || pref.view === 'network') && (
                                    <ApplicationResourceTree
                                        nodeFilter={(node) => this.filterTreeNode(node, treeFilter)}
                                        selectedNodeFullName={this.selectedNodeKey}
                                        onNodeClick={(fullName) => this.selectNode(fullName)}
                                        nodeMenu={(node) => this.renderResourceMenu(node, application)}
                                        tree={tree}
                                        app={application}
                                        useNetworkingHierarchy={pref.view === 'network'}
                                        onClearFilter={() => {
                                            this.appContext.apis.navigation.goto('.', { resource: '' } );
                                            services.viewPreferences.updatePreferences({ appDetails: { ...pref, resourceFilter: [] } });
                                        }}
                                        />
                                ) || (
                                    <div>
                                        {filteredRes.length > 0 && (
                                            <Paginate page={this.state.page} data={filteredRes} onPageChange={(page) => this.setState({page})} preferencesKey='application-details'>
                                            {(data) => (
                                                <ApplicationResourceList
                                                    onNodeClick={(fullName) => this.selectNode(fullName)}
                                                    resources={data}
                                                    nodeMenu={(node) => this.renderResourceMenu({...node, root: node}, application)}
                                                    />
                                                )}
                                            </Paginate>
                                        ) || (
                                            <EmptyState icon='fa fa-search'>
                                                <h4>No resources found</h4>
                                                <h5>Try to change filter criteria</h5>
                                            </EmptyState>
                                        )}
                                    </div>
                                )}
                            </div>
                            <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => this.selectNode('')}>
                                <div>
                                {selectedNode && (
                                    <DataLoader noLoaderOnInputChange={true} input={selectedNode.resourceVersion} load={async () => {
                                        const managedResources = await services.applications.managedResources(application.metadata.name);
                                        const controlled = managedResources.find((item) => isSameNode(selectedNode, item));
                                        const summary = application.status.resources.find((item) => isSameNode(selectedNode, item));
                                        const controlledState = controlled && summary && { summary, state: controlled } || null;
                                        const liveState = await services.applications.getResource(application.metadata.name, selectedNode).catch(() => null);
                                        const events = liveState && await services.applications.resourceEvents(application.metadata.name, {
                                            name: liveState.metadata.name,
                                            namespace: liveState.metadata.namespace,
                                            uid: liveState.metadata.uid,
                                        }) || [];

                                        return { controlledState, liveState, events };

                                    }}>{(data) =>
                                        <Tabs navTransparent={true} tabs={this.getResourceTabs(application, selectedNode, data.liveState, data.events, [
                                                {title: 'SUMMARY', key: 'summary', content: (
                                                    <ApplicationNodeInfo application={application} live={data.liveState} controlled={data.controlledState} node={selectedNode}/>
                                                ),
                                            }])} selectedTabKey={tab} onTabSelected={(selected) => this.appContext.apis.navigation.goto('.', {tab: selected})}/>
                                    }</DataLoader>
                                )}
                                {isAppSelected && (
                                    <Tabs navTransparent={true} tabs={[{
                                        title: 'SUMMARY', key: 'summary', content: <ApplicationSummary app={application} updateApp={(app) => this.updateApp(app)}/>,
                                    }, {
                                        title: 'PARAMETERS', key: 'parameters', content: (
                                            <DataLoader key='appDetails' input={{
                                                repoURL: application.spec.source.repoURL,
                                                path: application.spec.source.path,
                                                targetRevision: application.spec.source.targetRevision,
                                                details: { helm: application.spec.source.helm },
                                            }} load={(src) =>
                                                services.repos.appDetails(src.repoURL, src.path, src.targetRevision, src.details)
                                                .catch(() => ({ type: 'Directory' as appModels.AppSourceType, path: application.spec.source.path }))}>
                                            {(details: appModels.RepoAppDetails) => <ApplicationParameters
                                                    save={(app) => this.updateApp(app)} application={application} details={details} />}
                                            </DataLoader>
                                        ),
                                    }, {
                                        title: 'MANIFEST', key: 'manifest', content: (
                                            <YamlEditor minHeight={800} input={application.spec} onSave={async (patch) => {
                                                const spec = JSON.parse(JSON.stringify(application.spec));
                                                return services.applications.updateSpec(application.metadata.name, jsonMergePatch.apply(spec, JSON.parse(patch)));
                                            }}/>
                                        ),
                                    }, {
                                        icon: 'fa fa-file-medical', title: 'DIFF', key: 'diff', content: (
                                            <DataLoader key='diff'
                                                        load={async () => await services.applications.managedResources(application.metadata.name)}>{(managedResources) =>
                                                <ApplicationResourcesDiff states={managedResources}/>}</DataLoader>
                                        ),
                                    }, {
                                        title: 'EVENTS', key: 'event', content: <ApplicationResourceEvents applicationName={application.metadata.name}/>,
                                    }]} selectedTabKey={tab} onTabSelected={(selected) => this.appContext.apis.navigation.goto('.', {tab: selected})}/>
                                )}
                                </div>
                            </SlidingPanel>
                            <ApplicationSyncPanel
                                application={application}
                                hide={() => this.showDeploy(null)}
                                selectedResource={syncResourceKey}
                                />
                            <SlidingPanel isShown={this.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                                {this.selectedRollbackDeploymentIndex > -1 && <ApplicationDeploymentHistory
                                    app={application}
                                    selectedRollbackDeploymentIndex={this.selectedRollbackDeploymentIndex}
                                    rollbackApp={(info) => this.rollbackApplication(info)}
                                    selectDeployment={(i) => this.setRollbackPanelVisible(i)}
                                    />}
                            </SlidingPanel>
                            <SlidingPanel isShown={this.showOperationState && !!operationState} onClose={() => this.setOperationStatusVisible(false)}>
                                {operationState && <ApplicationOperationState  application={application} operationState={operationState}/>}
                            </SlidingPanel>
                            <SlidingPanel isShown={this.showConditions && !!conditions} onClose={() => this.setConditionsStatusVisible(false)}>
                                {conditions && <ApplicationConditions conditions={conditions}/>}
                            </SlidingPanel>
                        </Page>
                        </div>
                    );
                }}
            </DataLoader>)}
            </ObservableQuery>
        );
    }

    private getApplicationActionMenu(app: appModels.Application) {
        const refreshing = app.metadata.annotations && app.metadata.annotations[appModels.AnnotationRefreshKey];
        const fullName = nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace });
        return [{
            iconClassName: 'fa fa-info-circle',
            title: <span className='show-for-medium'>App Details</span>,
            action: () => this.selectNode(fullName),
        }, {
            iconClassName: 'fa fa-file-medical',
            title: <span className='show-for-medium'>App Diff</span>,
            action: () => this.selectNode(fullName, 0, 'diff'),
            disabled: app.status.sync.status === SyncStatuses.Synced,
        }, {
            iconClassName: 'fa fa-sync',
            title: <span className='show-for-medium'>Sync</span>,
            action: () => this.showDeploy('all'),
        }, {
            iconClassName: 'fa fa-info-circle',
            title: <span className='show-for-medium'>Sync Status</span>,
            action:  () => this.setOperationStatusVisible(true),
            disabled: !app.status.operationState,
        }, {
            iconClassName: 'fa fa-history',
            title: <span className='show-for-medium'>History and rollback</span>,
            action: () => this.setRollbackPanelVisible(0),
            disabled: !app.status.operationState,
        }, {
            iconClassName: 'fa fa-times-circle',
            title: <span className='show-for-medium'>Delete</span>,
            action: () => this.deleteApplication(),
        }, {
            iconClassName: classNames('fa fa-redo', { 'status-icon--spin': !!refreshing }),
            title: (
                <React.Fragment><span className='show-for-medium'>Refresh</span> <DropDownMenu items={[{
                    title: 'Hard Refresh', action: () => !refreshing && services.applications.get(app.metadata.name, 'hard'),
                }]} anchor={() => <i className='fa fa-caret-down'/>} /></React.Fragment>
            ),
            disabled: !!refreshing,
            action: () => !refreshing && services.applications.get(app.metadata.name, 'normal'),
        }];
    }

    private filterTreeNode(node: ResourceTreeNode, filter: {kind: string[], health: string[], sync: string[]}): boolean {
        const syncStatuses = filter.sync.map((item) => item === 'OutOfSync' ? ['OutOfSync', 'Unknown'] : [item] ).reduce(
            (first, second) => first.concat(second), []);
        const healthStatuses = filter.health.map((item) => item === 'Unhealthy' ? ['Unknown', 'Progressing', 'Degraded', 'Missing'] : [item] ).reduce(
            (first, second) => first.concat(second), []);

        return (filter.kind.length === 0 || filter.kind.indexOf(node.kind) > -1) &&
                (syncStatuses.length === 0 || node.root.hook ||  node.root.status && syncStatuses.indexOf(node.root.status) > -1) &&
                (healthStatuses.length === 0 || node.root.hook || node.root.health && healthStatuses.indexOf(node.root.health.status) > -1);
    }

    private loadAppInfo(name: string): Observable<{application: appModels.Application, tree: appModels.ApplicationTree}> {
        return Observable.merge(
            Observable.fromPromise(services.applications.get(name).then((app) => ({ app, watchEvent: false }))),
            services.applications.watch({name}).map((watchEvent) => {
                if (watchEvent.type === 'DELETED') {
                    this.onAppDeleted();
                }
                return { app: watchEvent.application, watchEvent: true };
            }).repeat().retryWhen((errors) => errors.delay(500)),
            this.refreshRequested.filter((e) => e !== null).flatMap(() => services.applications.get(name).then((app) => ({ app, watchEvent: true }))),
        ).flatMap((appInfo) => {
                const app = appInfo.app;
                const fallbackTree: appModels.ApplicationTree = {
                    nodes: app.status.resources.map((res) => ({...res, parentRefs: [], info: [], resourceVersion: '', uid: ''})),
                };
                const treeSource = new Observable<{ application: appModels.Application, tree: appModels.ApplicationTree }>((observer) => {
                    services.applications.resourceTree(app.metadata.name)
                        .then((tree) => observer.next({ application: app, tree }))
                        .catch((e) => {
                            observer.next({ application: app, tree: fallbackTree });
                            observer.error(e);
                        });
                }).repeat().retryWhen((errors) => errors.delay(1000));
                if (appInfo.watchEvent) {
                    return treeSource;
                } else {
                    return Observable.merge(Observable.from([{ application: app, tree: fallbackTree }]), treeSource);
                }
            });
    }

    private onAppDeleted() {
        this.appContext.apis.notifications.show({ type: NotificationType.Success, content: `Application '${this.props.match.params.name}' was deleted` });
        this.appContext.apis.navigation.goto('/applications');
    }

    private async updateApp(app: appModels.Application) {
        await services.applications.updateSpec(app.metadata.name, app.spec);
        this.refreshRequested.next({});
    }

    private groupAppNodesByKey(application: appModels.Application, tree: appModels.ApplicationTree) {
        const nodeByKey = new Map<string, appModels.ResourceDiff | appModels.ResourceNode | appModels.Application>();
        tree.nodes.forEach((node) => nodeByKey.set(nodeKey(node), node));
        nodeByKey.set(nodeKey({group: 'argoproj.io', kind: application.kind, name: application.metadata.name, namespace: application.metadata.namespace}), application);
        return nodeByKey;
    }

    private getTreeFilter(filter: string[]): {kind: string[], health: string[], sync: string[]} {
        const kind = new Array<string>();
        const health = new Array<string>();
        const sync = new Array<string>();
        for (const item of filter) {
            const [type, val] = item.split(':');
            switch (type) {
                case 'kind':
                    kind.push(val);
                    break;
                case 'health':
                    health.push(val);
                    break;
                case 'sync':
                    sync.push(val);
                    break;
            }
        }
        return {kind, health, sync};
    }

    private showDeploy(resource: string) {
        this.appContext.apis.navigation.goto('.', { deploy: resource });
    }

    private setOperationStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', { operation: isVisible });
    }

    private setConditionsStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', { conditions: isVisible });
    }

    private setRollbackPanelVisible(selectedDeploymentIndex = 0) {
        this.appContext.apis.navigation.goto('.', { rollback: selectedDeploymentIndex });
    }

    private selectNode(fullName: string, containerIndex = 0, tab: string = null) {
        const node = fullName ? `${fullName}/${containerIndex}` : null;
        this.appContext.apis.navigation.goto('.', { node, tab });
    }

    private async rollbackApplication(revisionHistory: appModels.RevisionHistory) {
        try {
            const confirmed = await this.appContext.apis.popup.confirm('Rollback application', `Are you sure you want to rollback application '${this.props.match.params.name}'?`);
            if (confirmed) {
                await services.applications.rollback(this.props.match.params.name, revisionHistory.id);
                this.refreshRequested.next({});
            }
            this.setRollbackPanelVisible(-1);
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to rollback application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private renderResourceMenu(resource: ResourceTreeNode, application: appModels.Application): React.ReactNode {
        let menuItems: Observable<MenuItem[]>;
        if (AppUtils.isAppNode(resource) && resource.name === application.metadata.name) {
            menuItems = Observable.from([this.getApplicationActionMenu(application)]);
        } else {
            const isRoot = resource.root && AppUtils.nodeKey(resource.root) === AppUtils.nodeKey(resource);
            const items: MenuItem[] = [...(isRoot && [{
                title: 'Sync',
                action: () => this.showDeploy(nodeKey(resource)),
            }] || []), {
                title: 'Delete',
                action: async () => {
                    this.appContext.apis.popup.prompt('Delete resource',
                    () => (
                        <div>
                            <p>Are your sure you want to delete {resource.kind} '{resource.name}'?`</p>
                            <div className='argo-form-row' style={{paddingLeft: '30px'}}>
                                <Checkbox id='force-delete-checkbox' field='force'/> <label htmlFor='force-delete-checkbox'>Force delete</label>
                            </div>
                        </div>
                    ),
                    {
                        submit: async (vals, _, close) => {
                            try {
                                await services.applications.deleteResource(this.props.match.params.name, resource, !!vals.force);
                                this.refreshRequested.next({});
                                close();
                            } catch (e) {
                                this.appContext.apis.notifications.show({
                                    content: <ErrorNotification title='Unable to delete resource' e={e}/>,
                                    type: NotificationType.Error,
                                });
                            }
                        },
                    });
                },
            }];
            const resourceActions = services.applications.getResourceActions(application.metadata.name, resource)
                .then((actions) => items.concat(actions.map((action) => ({
                    title: action.name,
                    action: async () => {
                        try {
                            const confirmed = await this.appContext.apis.popup.confirm(
                                `Execute '${action.name}' action?`, `Are you sure you want to execute '${action.name}' action?`);
                            if (confirmed) {
                                await services.applications.runResourceAction(application.metadata.name, resource, action.name);
                            }
                        } catch (e) {
                            this.appContext.apis.notifications.show({
                                content: <ErrorNotification title='Unable to execute resource action' e={e}/>,
                                type: NotificationType.Error,
                            });
                        }
                    },
                })))).catch(() => items);
            menuItems = Observable.merge(
                Observable.from([items]),
                Observable.fromPromise(resourceActions));
        }
        return (
            <DataLoader load={() => menuItems}>
            {(items) => (
                <ul>
                    {items.map((item, i) => (
                        <li style={{ textTransform: 'capitalize' }} key={i} onClick={(e) => {
                            e.stopPropagation();
                            item.action();
                            document.body.click(); // hack, trigger body click to make sure that dropdown
                        }}>{item.iconClassName && <i className={item.iconClassName}/>} {item.title}</li>
                    ))}
                </ul>
            )}
            </DataLoader>
        );
    }

    private async deleteApplication() {
        await AppUtils.deleteApplication(this.props.match.params.name, this.appContext.apis);
    }

    private getResourceTabs(application: appModels.Application, node: ResourceTreeNode, state: appModels.State, events: appModels.Event[], tabs: Tab[]) {
        if (state) {
            const numErrors = events.filter((event) => event.type !== 'Normal').reduce((total, event) => total + event.count, 0);
            tabs.push({
                title: 'EVENTS',
                badge: numErrors > 0 && numErrors || null,
                key: 'events', content: (
                    <div className='application-resource-events'>
                        <EventsList events={events}/>
                    </div>),
            });
        }
        if (node.kind === 'Pod' && state) {
            const containerGroups = [{
                offset: 0,
                title: 'INIT CONTAINERS',
                containers: state.spec.initContainers || [],
            }, {
                offset: (state.spec.initContainers || []).length,
                title: 'CONTAINERS',
                containers: state.spec.containers || [],
            }];
            tabs = tabs.concat([{
                key: 'logs',
                title: 'LOGS',
                content: (
                    <div className='application-details__tab-content-full-height'>
                        <div className='row'>
                            <div className='columns small-3 medium-2'>
                                {containerGroups.map((group) => (
                                    <div key={group.title} style={{marginBottom: '1em'}}>
                                        {group.containers.length > 0 && <p>{group.title}:</p>}
                                        {group.containers.map((container: any, i: number) => (
                                            <div className='application-details__container' key={container.name} onClick={() => this.selectNode(
                                                    this.selectedNodeKey, group.offset + i, 'logs')}>
                                                {(group.offset + i) === this.selectedNodeInfo.container && <i className='fa fa-angle-right'/>}
                                                <span title={container.name}>{container.name}</span>
                                            </div>
                                        ))}
                                    </div>
                                ))}
                            </div>
                            <div className='columns small-9 medium-10'>
                                <PodsLogsViewer
                                    pod={state} applicationName={application.metadata.name} containerIndex={this.selectedNodeInfo.container} />
                            </div>
                        </div>
                    </div>
                ),
            }]);
        }
        return tabs;
    }
}
