import {DropDownMenu, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import * as models from '../../../shared/models';
import {RouteComponentProps} from 'react-router';
import {BehaviorSubject, combineLatest, from, merge, Observable} from 'rxjs';
import {delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';

import {DataLoader, EmptyState, ErrorNotification, ObservableQuery, Page, Paginate, Revision, Timestamp} from '../../../shared/components';
import {AppContext, ContextApis} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {
    AbstractAppDetailsPreferences,
    AppDetailsPreferences,
    AppSetsDetailsViewKey,
    AppSetsDetailsViewType,
    AppsDetailsViewKey,
    AppsDetailsViewType,
    services
} from '../../../shared/services';

import {ApplicationConditions} from '../application-conditions/application-conditions';
import {ApplicationDeploymentHistory} from '../application-deployment-history/application-deployment-history';
import {ApplicationOperationState} from '../application-operation-state/application-operation-state';
import {PodGroupType, PodView} from '../application-pod-view/pod-view';
import {ApplicationResourceTree, ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ResourceDetails} from '../resource-details/resource-details';
import * as AppUtils from '../utils';
import {ApplicationResourceList} from './application-resource-list';
import {AbstractFiltersProps, Filters} from './application-resource-filter';
import {getAppDefaultSource, urlPattern, helpTip} from '../utils';
import {AbstractApplication, ApplicationTree, ChartDetails, ResourceStatus} from '../../../shared/models';
import {ApplicationsDetailsAppDropdown} from './application-details-app-dropdown';
import {useSidebarTarget} from '../../../sidebar/sidebar';

import './application-details.scss';
import {AppViewExtension, StatusPanelExtension} from '../../../shared/services/extensions-service';

interface ApplicationDetailsState {
    page: number;
    revision?: string;
    groupedResources?: ResourceStatus[];
    slidingPanelPage?: number;
    filteredGraph?: any[];
    truncateNameOnRight?: boolean;
    collapsedNodes?: string[];
    extensions?: AppViewExtension[];
    extensionsMap?: {[key: string]: AppViewExtension};
    statusExtensions?: StatusPanelExtension[];
    statusExtensionsMap?: {[key: string]: StatusPanelExtension};
}

interface FilterInput {
    name: string[];
    kind: string[];
    health: string[];
    sync: string[];
    namespace: string[];
}

const ApplicationDetailsFilters = (props: AbstractFiltersProps) => {
    const sidebarTarget = useSidebarTarget();
    return ReactDOM.createPortal(<Filters {...props} />, sidebarTarget?.current);
};

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
    appContext.navigation.goto('.', {node, tab}, {replace: true});
};

export abstract class AbstractApplicationDetails extends React.Component<RouteComponentProps<{appnamespace: string; name: string}> & {objectListKind: string}, ApplicationDetailsState> {
    public static contextTypes = {
        apis: PropTypes.object
    };

    protected appChanged = new BehaviorSubject<appModels.AbstractApplication>(null);
    protected appNamespace: string;
    protected objectListKind: string;
    protected isListOfApplications: boolean;

    constructor(props: RouteComponentProps<{appnamespace: string; name: string}> & {objectListKind: string}) {
        super(props);
        const extensions = services.extensions.getAppViewExtensions();
        const extensionsMap: {[key: string]: AppViewExtension} = {};
        extensions.forEach(ext => {
            extensionsMap[ext.title] = ext;
        });
        const statusExtensions = services.extensions.getStatusPanelExtensions();
        const statusExtensionsMap: {[key: string]: StatusPanelExtension} = {};
        statusExtensions.forEach(ext => {
            statusExtensionsMap[ext.id] = ext;
        });
        this.state = {
            page: 0,
            groupedResources: [],
            slidingPanelPage: 0,
            filteredGraph: [],
            truncateNameOnRight: false,
            collapsedNodes: [],
            extensions,
            extensionsMap,
            statusExtensions,
            statusExtensionsMap
        };
        if (typeof this.props.match.params.appnamespace === 'undefined') {
            this.appNamespace = '';
        } else {
            this.appNamespace = this.props.match.params.appnamespace;
        }
        this.isListOfApplications = this.props.objectListKind === "application";
    }

    protected get appContext(): AppContext {
        return this.context as AppContext;
    }

    private onAppDeleted() {
        this.appContext.apis.notifications.show({type: NotificationType.Success, content: `Application '${this.props.match.params.name}' was deleted`});
        this.appContext.apis.navigation.goto('/applications');
    }

    protected pageTitle: string;
    protected abstract getPageTitle(view: string): string;
    protected abstract getViewParam(params: URLSearchParams): string;
    protected abstract getApplicationActionMenu(app: appModels.AbstractApplication, needOverlapLabelOnNarrowScreen: boolean): ({ iconClassName: string; title: JSX.Element; action: () => void; disabled?: undefined; } | { iconClassName: string; title: JSX.Element; action: () => void; disabled: boolean; })[];
    protected abstract getDefaultView(application: AbstractApplication): string;
    protected abstract getOperationState(application: models.AbstractApplication): models.OperationState;
    protected abstract getConditions(application: models.AbstractApplication): models.ApplicationCondition[];

    protected loadAppInfo(name: string, appNamespace: string): Observable<{application: appModels.AbstractApplication; tree: appModels.AbstractApplicationTree}> {
        return from(services.applications.get(name, appNamespace, this.props.history.location.pathname))
            .pipe(
                mergeMap(app => {
                    const fallbackTree = this.getFallbackTree(app);
                    return combineLatest(
                        merge(
                            from([app]),
                            this.appChanged.pipe(filter(item => !!item)),
                            AppUtils.handlePageVisibility(() =>
                                services.applications
                                    .watch(this.props.history.location.pathname, {name, appNamespace})
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
                            services.applications.resourceTree(name, appNamespace, this.props.history.location.pathname).catch(() => fallbackTree),
                            AppUtils.handlePageVisibility(() =>
                                services.applications
                                    .watchResourceTree(name, appNamespace, this.props.history.location.pathname)
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

    // No differences
    protected getTreeFilter(filterInput: string[]): FilterInput {
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

    protected get selectedNodeInfo() {
        return NodeInfo(new URLSearchParams(this.props.history.location.search).get('node'));
    }

    protected get selectedNodeKey() {
        const nodeContainer = this.selectedNodeInfo;
        return nodeContainer.key;
    }

    protected filterTreeNode(node: ResourceTreeNode, filterInput: FilterInput): boolean {
        const syncStatuses = filterInput.sync.map(item => (item === 'OutOfSync' ? ['OutOfSync', 'Unknown'] : [item])).reduce((first, second) => first.concat(second), []);

        const root = node.root || ({} as ResourceTreeNode);
        const hook = root && root.hook;
        if (
            (filterInput.name.length === 0 || this.nodeNameMatchesWildcardFilters(node.name, filterInput.name)) &&
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

    private nodeNameMatchesWildcardFilters(nodeName: string, filterInputNames: string[]): boolean {
        const regularExpression = new RegExp(
            filterInputNames
                // Escape any regex input to ensure only * can be used
                .map(pattern => '^' + this.escapeRegex(pattern) + '$')
                // Replace any escaped * with proper regex
                .map(pattern => pattern.replace(/\\\*/g, '.*'))
                // Join all filterInputs to a single regular expression
                .join('|'),
            'gi'
        );
        return regularExpression.test(nodeName);
    }

    private escapeRegex(input: string): string {
        return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    protected get selectedExtension() {
        return new URLSearchParams(this.props.history.location.search).get('extension');
    }

    protected selectNode(fullName: string, containerIndex = 0, tab: string = null) {
        SelectNode(fullName, containerIndex, tab, this.appContext.apis);
    }

    protected setOperationStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', {operation: isVisible}, {replace: true});
    }

    protected setRollbackPanelVisible(selectedDeploymentIndex = 0) {
        this.appContext.apis.navigation.goto('.', {rollback: selectedDeploymentIndex}, {replace: true});
    }

    protected async deleteApplication() {
        await AppUtils.deleteApplication(this.props.match.params.name, this.appNamespace, this.appContext.apis);
    }

    protected setConditionsStatusVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', {conditions: isVisible}, {replace: true});
    }

    protected setExtensionPanelVisible(selectedExtension = '') {
        this.appContext.apis.navigation.goto('.', {extension: selectedExtension}, {replace: true});
    }

    protected toggleCompactView(app: models.Application, pref: AppDetailsPreferences) {
        pref.userHelpTipMsgs = pref.userHelpTipMsgs.map(usrMsg =>
            usrMsg.appName === app.metadata.name && usrMsg.msgKey === 'groupNodes' ? {...usrMsg, display: true} : usrMsg
        );
        services.viewPreferences.updatePreferences({appDetails: {...pref, groupNodes: !pref.groupNodes}});
    }

    protected setNodeExpansion(node: string, isExpanded: boolean) {
        const index = this.state.collapsedNodes.indexOf(node);
        if (isExpanded && index >= 0) {
            this.state.collapsedNodes.splice(index, 1);
            const updatedNodes = this.state.collapsedNodes.slice();
            this.setState({collapsedNodes: updatedNodes});
        } else if (!isExpanded && index < 0) {
            const updatedNodes = this.state.collapsedNodes.slice();
            updatedNodes.push(node);
            this.setState({collapsedNodes: updatedNodes});
        }
    }

    protected getNodeExpansion(node: string): boolean {
        return this.state.collapsedNodes.indexOf(node) < 0;
    }

    protected closeGroupedNodesPanel() {
        this.setState({groupedResources: []});
        this.setState({slidingPanelPage: 0});
    }

    protected async updateApp(app: appModels.Application, query: {validate?: boolean}) {
        const latestApp = await services.applications.get(app.metadata.name, app.metadata.namespace, this.props.history.location.pathname);
        latestApp.metadata.labels = app.metadata.labels;
        latestApp.metadata.annotations = app.metadata.annotations;
        latestApp.spec = app.spec;
        const updatedApp = await services.applications.update(latestApp, query);
        this.appChanged.next(updatedApp);
    }

    protected get selectedRollbackDeploymentIndex() {
        return parseInt(new URLSearchParams(this.props.history.location.search).get('rollback'), 10);
    }

    protected async rollbackApplication(revisionHistory: appModels.RevisionHistory, application: appModels.Application) {
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
                await services.applications.rollback(this.props.match.params.name, this.appNamespace, revisionHistory.id);
                this.appChanged.next(await services.applications.get(this.props.match.params.name, this.appNamespace, this.props.history.location.pathname));
                this.setRollbackPanelVisible(-1);
            }
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to rollback application' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    protected get showOperationState() {
        return new URLSearchParams(this.props.history.location.search).get('operation') === 'true';
    }

    protected get showConditions() {
        return new URLSearchParams(this.props.history.location.search).get('conditions') === 'true';
    }

    public render() {
        return (
            <ObservableQuery>
                {q => {
                    return (
                        <DataLoader
                            errorRenderer={error => (
                                <Page title={this.pageTitle}>{error}</Page>
                            )}
                            loadingRenderer={() => (
                                <Page title={this.pageTitle}>Loading...</Page>
                            )}
                            input={this.props.match.params.name}
                            load={name => combineLatest([this.loadAppInfo(name, this.appNamespace), services.viewPreferences.getPreferences(), q]).pipe(
                                map(items => {
                                    const application = items[0].application;
                                    const pref = items[1].appDetails;
                                    const params = items[2];
                                    if (params.get('resource') != null) {
                                        pref.resourceFilter = params
                                            .get('resource')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('view') != null) {
                                        pref.view = this.getViewParam(params);
                                    } else {
                                        const appDefaultView = this.getDefaultView(application);
                                        if (appDefaultView != null) {
                                            pref.view = appDefaultView;
                                        }
                                    }
                                    if (params.get('orphaned') != null) {
                                        pref.orphanedResources = params.get('orphaned') === 'true';
                                    }
                                    this.retrievePodSortMode(params, application, pref);
                                    return { ...items[0], pref };
                                })
                            )}>
                            {({
                                application, tree, pref
                            }: {
                                application: appModels.AbstractApplication;
                                tree: appModels.AbstractApplicationTree;
                                pref: AbstractAppDetailsPreferences;
                            }) => {
                                tree.nodes = tree.nodes || [];
                                const treeFilter = this.getTreeFilter(pref.resourceFilter);
                                const setFilter = (items: string[]) => {
                                    this.appContext.apis.navigation.goto('.', { resource: items.join(',') }, { replace: true });
                                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, resourceFilter: items } });
                                };
                                const clearFilter = () => setFilter([]);
                                const refreshing = application.metadata.annotations && application.metadata.annotations[appModels.AnnotationRefreshKey];
                                const appNodesByName = this.groupAppNodesByKey(application, tree);
                                const selectedItem = (this.selectedNodeKey && appNodesByName.get(this.selectedNodeKey)) || null;
                                const isAppSelected = selectedItem === application;
                                const selectedNode = !isAppSelected && (selectedItem as appModels.ResourceNode);
                                const operationState = this.getOperationState(application);
                                const conditions = this.getConditions(application);
                                const syncResourceKey = new URLSearchParams(this.props.history.location.search).get('deploy');
                                const tab = new URLSearchParams(this.props.history.location.search).get('tab');
                                const source = this.getSource(application);
                                const showToolTip = this.getShowToolTip(application, pref);
                                // const resourceNodes = (): any[] => {
                                //     const statusByKey = new Map<string, models.ResourceStatus>();
                                //     if (isApp(application)) {
                                //         (application as models.Application).status.resources.forEach(res => statusByKey.set(AppUtils.nodeKey(res), res));
                                //     }
                                //     const resources = new Map<string, any>();
                                //     tree.nodes
                                //         .map(node => ({...node, orphaned: false}))
                                //         .concat(
                                //             ((pref.orphanedResources && isApp(application) ? (tree as models.ApplicationTree).orphanedNodes : []) || []).map(node => ({
                                //                 ...node,
                                //                 orphaned: true
                                //             }))
                                //         )
                                //         .forEach(node => {
                                //             const resource: any = {...node};
                                //             resource.uid = node.uid;
                                //             const status = statusByKey.get(AppUtils.nodeKey(node));
                                //             if (status) {
                                //                 resource.health = status.health;
                                //                 resource.status = status.status;
                                //                 resource.hook = status.hook;
                                //                 resource.syncWave = status.syncWave;
                                //                 resource.requiresPruning = status.requiresPruning;
                                //             }
                                //             resources.set(node.uid || AppUtils.nodeKey(node), resource);
                                //         });
                                //     const resourcesRef = Array.from(resources.values());
                                //     return resourcesRef;
                                // };
                                const filteredRes = this.getResourceNodes(application, tree, pref).filter(res => {
                                    const resNode: ResourceTreeNode = { ...res, root: null, info: null, parentRefs: [], resourceVersion: '', uid: '' };
                                    resNode.root = resNode;
                                    return this.filterTreeNode(resNode, treeFilter);
                                });
                                const openGroupNodeDetails = (groupdedNodeIds: string[]) => {
                                    const resources = this.getResourceNodes(application, tree, pref);
                                    this.setState({
                                        groupedResources: groupdedNodeIds
                                            ? resources.filter(res => groupdedNodeIds.includes(res.uid) || groupdedNodeIds.includes(AppUtils.nodeKey(res)))
                                            : []
                                    });
                                };

                                const renderCommitMessage = (message: string) => message.split(/\s/).map(part => urlPattern.test(part) ? (
                                    <a href={part} target='_blank' rel='noopener noreferrer' style={{ overflowWrap: 'anywhere', wordBreak: 'break-word' }}>
                                        {part}{' '}
                                    </a>
                                ) : (
                                    part + ' '
                                )
                                );
                                const { Tree, Pods, Network, List } = AppsDetailsViewKey;
                                const zoomNum = (pref.zoom * 100).toFixed(0);
                                const setZoom = (s: number) => {
                                    let targetZoom: number = pref.zoom + s;
                                    if (targetZoom <= 0.05) {
                                        targetZoom = 0.1;
                                    } else if (targetZoom > 2.0) {
                                        targetZoom = 2.0;
                                    }
                                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, zoom: targetZoom } });
                                };
                                const setFilterGraph = (filterGraph: any[]) => {
                                    this.setState({ filteredGraph: filterGraph });
                                };
                                const setShowCompactNodes = (showCompactView: boolean) => {
                                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, groupNodes: showCompactView } });
                                };
                                const updateHelpTipState = (usrHelpTip: models.UserMessages) => {
                                    // if (isApp(application)) { // Could apply to appsets?
                                        const existingIndex = (pref as AppDetailsPreferences).userHelpTipMsgs.findIndex(
                                            msg => msg.appName === usrHelpTip.appName && msg.msgKey === usrHelpTip.msgKey
                                        );
                                        if (existingIndex !== -1) {
                                            (pref as AppDetailsPreferences).userHelpTipMsgs[existingIndex] = usrHelpTip;
                                        } else {
                                            ((pref as AppDetailsPreferences).userHelpTipMsgs || []).push(usrHelpTip);
                                        }
                                    // }
                                };
                                const toggleNameDirection = () => {
                                    this.setState({ truncateNameOnRight: !this.state.truncateNameOnRight });
                                };
                                const expandAll = () => {
                                    this.setState({ collapsedNodes: [] });
                                };
                                const collapseAll = () => {
                                    const nodes = new Array<ResourceTreeNode>();
                                    tree.nodes
                                        .map(node => ({ ...node, orphaned: false }))
                                        .concat((tree.orphanedNodes || []).map(node => ({ ...node, orphaned: true })))
                                        // We should have orphaned nodes for app sets?
                                        // .concat((isApp(application) ? (tree as models.ApplicationTree).orphanedNodes : [] || []).map(node => ({ ...node, orphaned: true })))
                                        .forEach(node => {
                                            const resourceNode: ResourceTreeNode = { ...node };
                                            nodes.push(resourceNode);
                                        });
                                    const collapsedNodesList = this.state.collapsedNodes.slice();
                                    if (pref.view === 'network') {
                                        const networkNodes = nodes.filter(node => node.networkingInfo);
                                        networkNodes.forEach(parent => {
                                            const parentId = parent.uid;
                                            if (collapsedNodesList.indexOf(parentId) < 0) {
                                                collapsedNodesList.push(parentId);
                                            }
                                        });
                                        this.setState({ collapsedNodes: collapsedNodesList });
                                    } else {
                                        const managedKeys = new Set(application.status.resources.map(AppUtils.nodeKey));
                                        nodes.forEach(node => {
                                            if (!((node.parentRefs || []).length === 0 || managedKeys.has(AppUtils.nodeKey(node)))) {
                                                node.parentRefs.forEach(parent => {
                                                    const parentId = parent.uid;
                                                    if (collapsedNodesList.indexOf(parentId) < 0) {
                                                        collapsedNodesList.push(parentId);
                                                    }
                                                });
                                            }
                                        });
                                        collapsedNodesList.push(application.kind + '-' + application.metadata.namespace + '-' + application.metadata.name);
                                        this.setState({ collapsedNodes: collapsedNodesList });
                                    }
                                };
                                const appFullName = AppUtils.nodeKey({
                                    group: 'argoproj.io',
                                    kind: application.kind,
                                    name: application.metadata.name,
                                    namespace: application.metadata.namespace
                                });

                                const activeExtension = this.state.statusExtensionsMap[this.selectedExtension];

                                return (
                                    <div className={`application-details ${this.props.match.params.name}`}>
                                        <Page
                                            title={this.props.match.params.name + ' - ' + this.getPageTitle(pref.view)}
                                            useTitleOnly={true}
                                            topBarTitle={this.getPageTitle(pref.view)}
                                            toolbar={{
                                                breadcrumbs: this.getBreadcrumb(application),
                                                actionMenu: { items: this.getApplicationActionMenu(application, true) },
                                                tools: (
                                                    <React.Fragment key='app-list-tools'>
                                                        <div className='application-details__view-type'>
                                                            {this.getViewButtons(pref, Tree, application, Pods, Network, List)}
                                                            {/* Extensions will show up in both application details and applicationset details? */}
                                                            {this.state.extensions &&
                                                                (this.state.extensions || []).map(ext => (
                                                                    <i
                                                                        key={ext.title}
                                                                        className={classNames(`fa ${ext.icon}`, { selected: pref.view === ext.title })}
                                                                        title={ext.title}
                                                                        onClick={() => {
                                                                            this.appContext.apis.navigation.goto('.', { view: ext.title });
                                                                            services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: ext.title } });
                                                                        } } />
                                                                )
                                                            )}
                                                        </div>
                                                    </React.Fragment>
                                                )
                                            }}>
                                            <div className='application-details__wrapper'>
                                                <div className='application-details__status-panel'>
                                                    <ApplicationStatusPanel
                                                        application={application}
                                                        showDiff={() => this.selectNode(appFullName, 0, 'diff')}
                                                        showOperation={() => this.setOperationStatusVisible(true)}
                                                        showConditions={() => this.setConditionsStatusVisible(true)}
                                                        showExtension={id => this.setExtensionPanelVisible(id)}
                                                        showMetadataInfo={revision => this.setState({ ...this.state, revision })} />
                                                </div>
                                                <div className='application-details__tree'>
                                                    {refreshing && <p className='application-details__refreshing-label'>Refreshing</p>}
                                                    {((pref.view === 'tree' || pref.view === 'network') && (
                                                        <>
                                                            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                {viewPref => (
                                                                    <ApplicationDetailsFilters
                                                                        pref={pref}
                                                                        tree={tree}
                                                                        onSetFilter={setFilter}
                                                                        onClearFilter={clearFilter}
                                                                        collapsed={viewPref.hideSidebar}
                                                                        resourceNodes={this.state.filteredGraph} />
                                                                )}
                                                            </DataLoader>
                                                            <div className='graph-options-panel'>
                                                                <a
                                                                    className={`group-nodes-button`}
                                                                    onClick={() => {
                                                                        toggleNameDirection();
                                                                    } }
                                                                    title={this.state.truncateNameOnRight ? 'Truncate resource name right' : 'Truncate resource name left'}>
                                                                    <i
                                                                        className={classNames({
                                                                            'fa fa-align-right': this.state.truncateNameOnRight,
                                                                            'fa fa-align-left': !this.state.truncateNameOnRight
                                                                        })} />
                                                                </a>
                                                                {(pref.view === 'tree' || pref.view === 'network') && (
                                                                    <Tooltip
                                                                        content={AppUtils.userMsgsList[showToolTip?.msgKey] || 'Group Nodes'}
                                                                        visible={pref.groupNodes && showToolTip !== undefined && !showToolTip?.display}
                                                                        duration={showToolTip?.duration}
                                                                        zIndex={1}>
                                                                        <a
                                                                            className={`group-nodes-button group-nodes-button${!pref.groupNodes ? '' : '-on'}`}
                                                                            title={pref.view === 'tree' ? 'Group Nodes' : 'Collapse Pods'}
                                                                            onClick={() => this.toggleCompactView(application as models.Application, pref as AppDetailsPreferences)}>
                                                                            <i className={classNames('fa fa-object-group fa-fw')} />
                                                                        </a>
                                                                    </Tooltip>
                                                                )}

                                                                <span className={`separator`} />
                                                                <a className={`group-nodes-button`} onClick={() => expandAll()} title='Expand all child nodes of all parent nodes'>
                                                                    <i className='fa fa-plus fa-fw' />
                                                                </a>
                                                                <a className={`group-nodes-button`} onClick={() => collapseAll()} title='Collapse all child nodes of all parent nodes'>
                                                                    <i className='fa fa-minus fa-fw' />
                                                                </a>
                                                                <span className={`separator`} />
                                                                <span>
                                                                    <a className={`group-nodes-button`} onClick={() => setZoom(0.1)} title='Zoom in'>
                                                                        <i className='fa fa-search-plus fa-fw' />
                                                                    </a>
                                                                    <a className={`group-nodes-button`} onClick={() => setZoom(-0.1)} title='Zoom out'>
                                                                        <i className='fa fa-search-minus fa-fw' />
                                                                    </a>
                                                                    <div className={`zoom-value`}>{zoomNum}%</div>
                                                                </span>
                                                            </div>
                                                            <ApplicationResourceTree
                                                                nodeFilter={node => this.filterTreeNode(node, treeFilter)}
                                                                selectedNodeFullName={this.selectedNodeKey}
                                                                onNodeClick={fullName => this.selectNode(fullName)}
                                                                nodeMenu={node => AppUtils.renderResourceMenu(
                                                                    node,
                                                                    application,
                                                                    tree as ApplicationTree,
                                                                    this.appContext.apis,
                                                                    this.props.history,
                                                                    this.appChanged,
                                                                    () => this.getApplicationActionMenu(application, false)
                                                                )}
                                                                showCompactNodes={pref.groupNodes}
                                                                // userMsgs={isApp(application) ? (pref as AppDetailsPreferences).userHelpTipMsgs : []}
                                                                userMsgs={(pref as AppDetailsPreferences).userHelpTipMsgs}
                                                                tree={tree}
                                                                app={application}
                                                                showOrphanedResources={pref.orphanedResources}
                                                                useNetworkingHierarchy={pref.view === 'network'}
                                                                onClearFilter={clearFilter}
                                                                onGroupdNodeClick={groupdedNodeIds => openGroupNodeDetails(groupdedNodeIds)}
                                                                zoom={pref.zoom}
                                                                // podGroupCount={isApp(application) ? (pref as AppDetailsPreferences).podGroupCount : 0}
                                                                appContext={this.appContext}
                                                                nameDirection={this.state.truncateNameOnRight}
                                                                filters={pref.resourceFilter}
                                                                setTreeFilterGraph={setFilterGraph}
                                                                updateUsrHelpTipMsgs={updateHelpTipState}
                                                                setShowCompactNodes={setShowCompactNodes}
                                                                setNodeExpansion={(node, isExpanded) => this.setNodeExpansion(node, isExpanded)}
                                                                getNodeExpansion={node => this.getNodeExpansion(node)} />
                                                        </>
                                                    )) ||
                                                        (pref.view === 'pods' && (
                                                            <PodView
                                                                tree={tree as ApplicationTree}
                                                                app={application}
                                                                onItemClick={fullName => this.selectNode(fullName)}
                                                                nodeMenu={node => AppUtils.renderResourceMenu(
                                                                    node,
                                                                    application,
                                                                    tree as ApplicationTree,
                                                                    this.appContext.apis,
                                                                    this.props.history,
                                                                    this.appChanged,
                                                                    () => this.getApplicationActionMenu(application, false)
                                                                )}
                                                                quickStarts={node => AppUtils.renderResourceButtons(
                                                                    node,
                                                                    application,
                                                                    tree as ApplicationTree,
                                                                    this.appContext.apis,
                                                                    this.props.history,
                                                                    this.appChanged
                                                                )} />
                                                        )) ||
                                                        (this.state.extensionsMap[pref.view] != null && (
                                                            <ExtensionView extension={this.state.extensionsMap[pref.view]} application={application} tree={tree as ApplicationTree} />
                                                        )) || (
                                                            <div>
                                                                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                    {viewPref => (
                                                                        <ApplicationDetailsFilters
                                                                            pref={pref}
                                                                            tree={tree}
                                                                            onSetFilter={setFilter}
                                                                            onClearFilter={clearFilter}
                                                                            collapsed={viewPref.hideSidebar}
                                                                            resourceNodes={filteredRes} />
                                                                    )}
                                                                </DataLoader>
                                                                {(filteredRes.length > 0 && (
                                                                    <Paginate
                                                                        page={this.state.page}
                                                                        data={filteredRes}
                                                                        onPageChange={page => this.setState({ page })}
                                                                        preferencesKey='application-details'>
                                                                        {data => (
                                                                            <ApplicationResourceList
                                                                                onNodeClick={fullName => this.selectNode(fullName)}
                                                                                resources={data}
                                                                                nodeMenu={node => AppUtils.renderResourceMenu(
                                                                                    { ...node, root: node },
                                                                                    application,
                                                                                    tree as ApplicationTree,
                                                                                    this.appContext.apis,
                                                                                    this.props.history,
                                                                                    this.appChanged,
                                                                                    () => this.getApplicationActionMenu(application, false)
                                                                                )}
                                                                                tree={tree as ApplicationTree} />
                                                                        )}
                                                                    </Paginate>
                                                                )) || (
                                                                        <EmptyState icon='fa fa-search'>
                                                                            <h4>No resources found</h4>
                                                                            <h5>Try to change filter criteria</h5>
                                                                        </EmptyState>
                                                                    )}
                                                            </div>
                                                        )}
                                                </div>
                                            </div>
                                            <SlidingPanel isShown={this.state.groupedResources.length > 0} onClose={() => this.closeGroupedNodesPanel()}>
                                                <div className='application-details__sliding-panel-pagination-wrap'>
                                                    <Paginate
                                                        page={this.state.slidingPanelPage}
                                                        data={this.state.groupedResources}
                                                        onPageChange={page => this.setState({ slidingPanelPage: page })}
                                                        preferencesKey='grouped-nodes-details'>
                                                        {data => (
                                                            <ApplicationResourceList
                                                                onNodeClick={fullName => this.selectNode(fullName)}
                                                                resources={data}
                                                                nodeMenu={node => AppUtils.renderResourceMenu(
                                                                    { ...node, root: node },
                                                                    application,
                                                                    tree as ApplicationTree,
                                                                    this.appContext.apis,
                                                                    this.props.history,
                                                                    this.appChanged,
                                                                    () => this.getApplicationActionMenu(application, false)
                                                                )}
                                                                tree={tree as ApplicationTree} />
                                                        )}
                                                    </Paginate>
                                                </div>
                                            </SlidingPanel>
                                            <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => this.selectNode('')}>
                                                <ResourceDetails
                                                    tree={tree as ApplicationTree}
                                                    application={application}
                                                    isAppSelected={isAppSelected}
                                                    updateApp={(app: models.Application, query: { validate?: boolean; }) => this.updateApp(app, query)}
                                                    selectedNode={selectedNode}
                                                    tab={tab} />
                                            </SlidingPanel>
                                            <ApplicationSyncPanel
                                                application={application}
                                                hide={() => AppUtils.showDeploy(null, null, this.appContext.apis)}
                                                selectedResource={syncResourceKey} />
                                            <SlidingPanel isShown={this.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                                                {this.selectedRollbackDeploymentIndex > -1 && (
                                                    <ApplicationDeploymentHistory
                                                        app={application}
                                                        selectedRollbackDeploymentIndex={this.selectedRollbackDeploymentIndex}
                                                        rollbackApp={info => this.rollbackApplication(info, application)}
                                                        selectDeployment={i => this.setRollbackPanelVisible(i)} />
                                                )}
                                            </SlidingPanel>
                                            <SlidingPanel isShown={this.showOperationState && !!operationState} onClose={() => this.setOperationStatusVisible(false)}>
                                                {operationState && <ApplicationOperationState application={application} operationState={operationState} />}
                                            </SlidingPanel>
                                            <SlidingPanel isShown={this.showConditions && !!conditions} onClose={() => this.setConditionsStatusVisible(false)}>
                                                {conditions && <ApplicationConditions conditions={conditions} />}
                                            </SlidingPanel>
                                            <SlidingPanel isShown={!!this.state.revision} isMiddle={true} onClose={() => this.setState({ revision: null })}>
                                                {this.state.revision &&
                                                    (source.chart ? (
                                                        <DataLoader
                                                            input={application}
                                                            load={input => services.applications.revisionChartDetails(input.metadata.name, input.metadata.namespace, this.state.revision)}>
                                                            {(m: ChartDetails) => (
                                                                <div className='white-box' style={{ marginTop: '1.5em' }}>
                                                                    <div className='white-box__details'>
                                                                        <div className='row white-box__details-row'>
                                                                            <div className='columns small-3'>Revision:</div>
                                                                            <div className='columns small-9'>{this.state.revision}</div>
                                                                        </div>
                                                                        <div className='row white-box__details-row'>
                                                                            <div className='columns small-3'>Helm Chart:</div>
                                                                            <div className='columns small-9'>
                                                                                {source.chart}&nbsp;
                                                                                {m.home && (
                                                                                    <a
                                                                                        title={m.home}
                                                                                        onClick={e => {
                                                                                            e.stopPropagation();
                                                                                            window.open(m.home);
                                                                                        } }>
                                                                                        <i className='fa fa-external-link-alt' />
                                                                                    </a>
                                                                                )}
                                                                            </div>
                                                                        </div>
                                                                        {m.description && (
                                                                            <div className='row white-box__details-row'>
                                                                                <div className='columns small-3'>Description:</div>
                                                                                <div className='columns small-9'>{m.description}</div>
                                                                            </div>
                                                                        )}
                                                                        {m.maintainers && m.maintainers.length > 0 && (
                                                                            <div className='row white-box__details-row'>
                                                                                <div className='columns small-3'>Maintainers:</div>
                                                                                <div className='columns small-9'>{m.maintainers.join(', ')}</div>
                                                                            </div>
                                                                        )}
                                                                    </div>
                                                                </div>
                                                            )}
                                                        </DataLoader>
                                                    ) : (
                                                        <DataLoader
                                                            load={() => services.applications.revisionMetadata(application.metadata.name, application.metadata.namespace, this.state.revision)}>
                                                            {metadata => (
                                                                <div className='white-box' style={{ marginTop: '1.5em' }}>
                                                                    <div className='white-box__details'>
                                                                        <div className='row white-box__details-row'>
                                                                            <div className='columns small-3'>SHA:</div>
                                                                            <div className='columns small-9'>
                                                                                <Revision repoUrl={source.repoURL} revision={this.state.revision} />
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
                                                                            <div className='columns small-9' style={{ display: 'flex', alignItems: 'center' }}>
                                                                                <div className='application-details__commit-message'>{renderCommitMessage(metadata.message)}</div>
                                                                            </div>
                                                                        </div>
                                                                    </div>
                                                                </div>
                                                            )}
                                                        </DataLoader>
                                                    ))}
                                            </SlidingPanel>
                                            <SlidingPanel
                                                isShown={this.selectedExtension !== '' && activeExtension != null && activeExtension.flyout != null}
                                                onClose={() => this.setExtensionPanelVisible('')}>
                                                {this.selectedExtension !== '' && activeExtension && activeExtension.flyout && (
                                                    <activeExtension.flyout application={application} tree={tree} />
                                                )}
                                            </SlidingPanel>
                                        </Page>
                                    </div>
                                );
                            }}
                        </DataLoader>
                    );
                }}
            </ObservableQuery>
        );
    }

    protected abstract retrievePodSortMode(params: URLSearchParams, application: models.AbstractApplication, pref: AbstractAppDetailsPreferences): void;

    protected abstract groupAppNodesByKey(application: appModels.AbstractApplication, tree: appModels.AbstractApplicationTree): Map<string, models.AbstractApplication | models.ResourceDiff | models.ResourceNode>;

    protected abstract getFallbackTree(app: models.AbstractApplication): models.AbstractApplicationTree;

    protected abstract getViewButtons(pref: AbstractAppDetailsPreferences, Tree: AppsDetailsViewKey, application: models.AbstractApplication, Pods: AppsDetailsViewKey, Network: AppsDetailsViewKey, List: AppsDetailsViewKey): JSX.Element;

    protected abstract getBreadcrumb(application: models.AbstractApplication): { title: React.ReactNode; path?: string; }[];

    protected abstract getShowToolTip(application: models.AbstractApplication, pref: AbstractAppDetailsPreferences): models.UserMessages;

    protected abstract getSource(application: models.AbstractApplication): models.ApplicationSource;

    protected abstract getResourceNodes(abstractApplication: AbstractApplication, tree: models.AbstractApplicationTree, pref: AbstractAppDetailsPreferences): any[];
}

export class ApplicationDetailsForApplicationSets extends AbstractApplicationDetails {

    protected pageTitle = "ApplicationSet Details";

    protected getPageTitle(view: string): string {
        const {Tree, List} = AppSetsDetailsViewKey;
        switch (view) {
            case Tree:
                return 'ApplicationSet Details Tree';
            case List:
                return 'ApplicationSet Details List';
        }
        return '';
    }

    protected getViewParam(params: URLSearchParams): string {
        return (params.get('view') as AppSetsDetailsViewType);
    }

    protected getDefaultView(application: models.AbstractApplication): string {
         return ((application.metadata &&
                  application.metadata.annotations &&
                  application.metadata.annotations[appModels.AnnotationDefaultView]) as AppSetsDetailsViewType);
    }

    protected groupAppNodesByKey(application: appModels.AbstractApplication, tree: appModels.AbstractApplicationTree): Map<string, models.AbstractApplication | models.ResourceDiff | models.ResourceNode> {
        const nodeByKey = new Map<string, appModels.ResourceDiff | appModels.ResourceNode | appModels.AbstractApplication>();
        tree.nodes.concat([]).forEach(node => nodeByKey.set(AppUtils.nodeKey(node), node));
        nodeByKey.set(AppUtils.nodeKey({group: 'argoproj.io', kind: application.kind, name: application.metadata.name, namespace: application.metadata.namespace}), application);
        return nodeByKey;
    }

    protected getApplicationActionMenu(app: appModels.AbstractApplication, needOverlapLabelOnNarrowScreen: boolean): ({ iconClassName: string; title: JSX.Element; action: () => void; disabled?: undefined; } | { iconClassName: string; title: JSX.Element; action: () => void; disabled: boolean; })[] {
        const fullName = AppUtils.nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
        const ActionMenuItem = (prop: {actionLabel: string}) => <span className={needOverlapLabelOnNarrowScreen ? 'show-for-large' : ''}>{prop.actionLabel}</span>;
        return [
            {
                iconClassName: 'fa fa-info-circle',
                title: <ActionMenuItem actionLabel='AppSet Details' />,
                action: () => this.selectNode(fullName)
            }
        ];
    }

    protected getResourceNodes(abstractApplication: AbstractApplication, tree: models.AbstractApplicationTree, pref: AbstractAppDetailsPreferences): any[] {
        // Need to implement
        const resources = new Map<string, any>();
        const resourcesRef = Array.from(resources.values());
        return resourcesRef;
    };

    protected retrievePodSortMode(params: URLSearchParams, application: models.AbstractApplication, pref: AbstractAppDetailsPreferences): void {
       
    }

    protected getFallbackTree(app: models.AbstractApplication): models.AbstractApplicationTree {
        return {
            nodes: (app as models.ApplicationSet).status.resources.map(res => ({ ...res, parentRefs: [], info: [], resourceVersion: '', uid: '' })),
            orphanedNodes: []
        }
    }

    protected getOperationState(application: models.AbstractApplication): models.OperationState {
        return null;
    }

    protected getConditions(application: models.AbstractApplication): models.ApplicationCondition[] {
        return [];
    }

    protected getSource(application: models.AbstractApplication): models.ApplicationSource {
        return null;
    }

    protected getShowToolTip(application: models.AbstractApplication, pref: AbstractAppDetailsPreferences): models.UserMessages {
        return null;
    }

    protected getBreadcrumb(application: models.AbstractApplication): { title: React.ReactNode; path?: string; }[] {
        return [
            { title: 'ApplicationSets', path: '/settings/applicationsets' },
            { title: <ApplicationsDetailsAppDropdown appName={this.props.match.params.name} /> }
        ];
    }

    protected getViewButtons(pref: AbstractAppDetailsPreferences, Tree: AppsDetailsViewKey, application: models.AbstractApplication, Pods: AppsDetailsViewKey, Network: AppsDetailsViewKey, List: AppsDetailsViewKey): JSX.Element {
        return <>
            <i
                className={classNames('fa fa-sitemap', { selected: pref.view === Tree })}
                title='Tree'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: Tree });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: Tree } });
                } } />
            <i
                className={classNames('fa fa-th-list', { selected: pref.view === List })}
                title='List'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: List });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: List } });
                } } />
        </>;
    }
}

export class ApplicationDetailsForApplications extends AbstractApplicationDetails {

    protected pageTitle = "Application Details";

    protected getPageTitle(view: string): string {
        const {Tree, Pods, Network, List} = AppsDetailsViewKey;
        switch (view) {
            case Tree:
                return 'Application Details Tree';
            case Network:
                return 'Application Details Network';
            case Pods:
                return 'Application Details Pods';
            case List:
                return 'Application Details List';
        }
        return '';
    }

    protected getViewParam(params: URLSearchParams): string {
        return (params.get('view') as AppsDetailsViewType);
    }

    protected getDefaultView(application: models.AbstractApplication): string {
        return ((application.metadata &&
                  application.metadata.annotations &&
                  application.metadata.annotations[appModels.AnnotationDefaultView]) as AppsDetailsViewType)
    }

    protected groupAppNodesByKey(application: appModels.AbstractApplication, tree: appModels.AbstractApplicationTree): Map<string, models.AbstractApplication | models.ResourceDiff | models.ResourceNode> {
        const nodeByKey = new Map<string, appModels.ResourceDiff | appModels.ResourceNode | appModels.AbstractApplication>();
        tree.nodes.concat(((tree as appModels.ApplicationTree).orphanedNodes ) || []).forEach(node => nodeByKey.set(AppUtils.nodeKey(node), node));
        nodeByKey.set(AppUtils.nodeKey({group: 'argoproj.io', kind: application.kind, name: application.metadata.name, namespace: application.metadata.namespace}), application);
        return nodeByKey;
    }

    protected getApplicationActionMenu(app: appModels.AbstractApplication, needOverlapLabelOnNarrowScreen: boolean): ({ iconClassName: string; title: JSX.Element; action: () => void; disabled?: undefined; } | { iconClassName: string; title: JSX.Element; action: () => void; disabled: boolean; })[] {
        const refreshing = app.metadata.annotations && app.metadata.annotations[appModels.AnnotationRefreshKey];
        const fullName = AppUtils.nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
        const ActionMenuItem = (prop: {actionLabel: string}) => <span className={needOverlapLabelOnNarrowScreen ? 'show-for-large' : ''}>{prop.actionLabel}</span>;
        const hasMultipleSources = app.spec.sources && app.spec.sources.length > 0;
        return [
                  {
                      iconClassName: 'fa fa-info-circle',
                      title: <ActionMenuItem actionLabel='Details' />,
                      action: () => this.selectNode(fullName)
                  },
                  {
                      iconClassName: 'fa fa-file-medical',
                      title: <ActionMenuItem actionLabel='Diff' />,
                      action: () => this.selectNode(fullName, 0, 'diff'),
                      disabled: (app as models.Application).status.sync.status === appModels.SyncStatuses.Synced
                  },
                  {
                      iconClassName: 'fa fa-sync',
                      title: <ActionMenuItem actionLabel='Sync' />,
                      action: () => AppUtils.showDeploy('all', null, this.appContext.apis)
                  },
                  {
                      iconClassName: 'fa fa-info-circle',
                      title: <ActionMenuItem actionLabel='Sync Status' />,
                      action: () => this.setOperationStatusVisible(true),
                      disabled: !(app as models.Application).status.operationState
                  },
                  {
                      iconClassName: 'fa fa-history',
                      title: hasMultipleSources ? (
                          <React.Fragment>
                              <ActionMenuItem actionLabel=' History and rollback' />
                              {helpTip('Rollback is not supported for apps with multiple sources')}
                          </React.Fragment>
                      ) : (
                          <ActionMenuItem actionLabel='History and rollback' />
                      ),
                      action: () => {
                          this.setRollbackPanelVisible(0);
                      },
                      disabled: !(app as models.Application).status.operationState || hasMultipleSources
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
                                          action: () =>
                                              !refreshing && services.applications.get(app.metadata.name, app.metadata.namespace, this.props.history.location.pathname, 'hard')
                                      }
                                  ]}
                                  anchor={() => <i className='fa fa-caret-down' />}
                              />
                          </React.Fragment>
                      ),
                      disabled: !!refreshing,
                      action: () => {
                          if (!refreshing) {
                              services.applications.get(app.metadata.name, app.metadata.namespace, this.props.location.pathname, 'normal');
                              AppUtils.setAppRefreshing(app);
                              this.appChanged.next(app);
                          }
                      }
                  }
        ];
    }

    protected getResourceNodes(abstractApplication: AbstractApplication, tree: models.AbstractApplicationTree, pref: AbstractAppDetailsPreferences): any[] {
        const statusByKey = new Map<string, models.ResourceStatus>();
        const application = abstractApplication as models.Application;
        application.status.resources.forEach(res => statusByKey.set(AppUtils.nodeKey(res), res));
        const resources = new Map<string, any>();
        tree.nodes
            .map(node => ({...node, orphaned: false}))
            .concat(
                ((pref.orphanedResources && (tree as models.ApplicationTree).orphanedNodes) || []).map(node => ({
                    ...node,
                    orphaned: true
                }))
            )
            .forEach(node => {
                const resource: any = {...node};
                resource.uid = node.uid;
                const status = statusByKey.get(AppUtils.nodeKey(node));
                if (status) {
                    resource.health = status.health;
                    resource.status = status.status;
                    resource.hook = status.hook;
                    resource.syncWave = status.syncWave;
                    resource.requiresPruning = status.requiresPruning;
                }
                resources.set(node.uid || AppUtils.nodeKey(node), resource);
            });
        const resourcesRef = Array.from(resources.values());
        return resourcesRef;
    };

    protected retrievePodSortMode(params: URLSearchParams, application: models.AbstractApplication, pref: AbstractAppDetailsPreferences) {
        if (params.get('podSortMode') != null) {
            (pref as AppDetailsPreferences).podView.sortMode = params.get('podSortMode') as PodGroupType;
        } else {
            const appDefaultPodSort = (application.metadata &&
                application.metadata.annotations &&
                application.metadata.annotations[appModels.AnnotationDefaultPodSort]) as PodGroupType;
            if (appDefaultPodSort != null) {
                (pref as AppDetailsPreferences).podView.sortMode = appDefaultPodSort;
            }
        }
    }

    protected getFallbackTree(app: models.AbstractApplication): models.AbstractApplicationTree {
        return {
            nodes: 
                 (app as models.Application).status.resources.map(res => ({ ...res, parentRefs: [], info: [], resourceVersion: '', uid: '' })),
            orphanedNodes: [],
            hosts: []
        } as appModels.AbstractApplicationTree;
    }

    protected getOperationState(application: models.AbstractApplication): models.OperationState {
        return (application as models.Application).status.operationState;
    }

    protected getConditions(application: models.AbstractApplication): models.ApplicationCondition[] {
        return (application as models.Application).status.conditions || [];
    }

    protected getSource(application: models.AbstractApplication): models.ApplicationSource {
        return getAppDefaultSource(application as models.Application);
    }

    protected getShowToolTip(application: models.AbstractApplication, pref: AbstractAppDetailsPreferences): models.UserMessages {
        return (pref as AppDetailsPreferences)?.userHelpTipMsgs.find(usrMsg => usrMsg.appName === application.metadata.name);
    }

    protected getBreadcrumb(application: models.AbstractApplication): { title: React.ReactNode; path?: string; }[] {
        return [
            { title: 'Applications', path: '/applications' },
            { title: <ApplicationsDetailsAppDropdown appName={this.props.match.params.name} /> }
        ];
    }

    protected getViewButtons(pref: AbstractAppDetailsPreferences, Tree: AppsDetailsViewKey, application: models.AbstractApplication, Pods: AppsDetailsViewKey, Network: AppsDetailsViewKey, List: AppsDetailsViewKey): JSX.Element {
        return <>
            <i
                className={classNames('fa fa-sitemap', { selected: pref.view === Tree })}
                title='Tree'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: Tree });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: Tree } });
                } } />
            <i
                className={classNames('fa fa-th', { selected: pref.view === Pods })}
                title='Pods'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: Pods });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: Pods } });
                }} />
            <i
                className={classNames('fa fa-network-wired', { selected: pref.view === Network })}
                title='Network'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: Network });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: Network } });
                }} />
            <i
                className={classNames('fa fa-th-list', { selected: pref.view === List })}
                title='List'
                onClick={() => {
                    this.appContext.apis.navigation.goto('.', { view: List });
                    services.viewPreferences.updatePreferences({ appDetails: { ...pref, view: List } });
                }} />            
        </>;
    }
}

const ExtensionView = (props: {extension: AppViewExtension; application: models.Application; tree: models.ApplicationTree}) => {
    const {extension, application, tree} = props;
    return <extension.component application={application} tree={tree} />;
};
