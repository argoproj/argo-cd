import { FormField, MenuItem, NotificationType, SlidingPanel, Tab, Tabs, TopBarFilter } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Checkbox, Form, FormApi, Text } from 'react-form';
import { RouteComponentProps } from 'react-router';
import { Observable, Subscription } from 'rxjs';

import { DataLoader, ErrorNotification, Page } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as appModels from '../../../shared/models';
import { services } from '../../../shared/services';

import { ApplicationConditions } from '../application-conditions/application-conditions';
import { ApplicationDeploymentHistory } from '../application-deployment-history/application-deployment-history';
import { ApplicationNodeInfo } from '../application-node-info/application-node-info';
import { ApplicationOperationState } from '../application-operation-state/application-operation-state';
import { ApplicationResourceEvents } from '../application-resource-events/application-resource-events';
import { ApplicationResourcesTree } from '../application-resources-tree/application-resources-tree';
import { ApplicationStatusPanel } from '../application-status-panel/application-status-panel';
import { ApplicationSummary } from '../application-summary/application-summary';
import { ParametersPanel } from '../parameters-panel/parameters-panel';
import { PodsLogsViewer } from '../pod-logs-viewer/pod-logs-viewer';
import * as AppUtils from '../utils';

require('./application-details.scss');

// ContainerStatus holds just the state of a container status, used for type-checking.
interface ContainerStatus {
    state: {[name: string]: any};
}

export class ApplicationDetails extends React.Component<RouteComponentProps<{ name: string; }>, { defaultKindFilter: string[], refreshing: boolean}> {

    public static contextTypes = {
        apis: PropTypes.object,
    };

    private viewPrefSubscription: Subscription;
    private appUpdates: Observable<appModels.Application>;
    private formApi: FormApi;
    private loader: DataLoader<appModels.Application>;

    constructor(props: RouteComponentProps<{ name: string; }>) {
        super(props);
        this.state = { defaultKindFilter: [], refreshing: false };
        this.appUpdates = services.applications.watch({name: props.match.params.name}).map((changeEvent) => changeEvent.application);
    }

    private get showOperationState() {
        return new URLSearchParams(this.props.history.location.search).get('operation') === 'true';
    }

    private get showConditions() {
        return new URLSearchParams(this.props.history.location.search).get('conditions') === 'true';
    }

    private get showDeployPanel() {
        return new URLSearchParams(this.props.history.location.search).get('deploy') === 'true';
    }

    private get selectedRollbackDeploymentIndex() {
        return parseInt(new URLSearchParams(this.props.history.location.search).get('rollback'), 10);
    }

    private get selectedNodeContainer() {
        const nodeContainer = {
            fullName: '',
            container: 0,
        };
        const node = new URLSearchParams(this.props.location.search).get('node');
        if (node) {
            const parts = node.split(':');
            nodeContainer.fullName = parts.slice(0, 2).join(':');
            nodeContainer.container = parseInt(parts[2] || '0', 10);
        }
        return nodeContainer;
    }

    private get selectedNodeFullName() {
        const nodeContainer = this.selectedNodeContainer;
        return nodeContainer.fullName;
    }

    public async componentDidMount() {
        this.ensureUnsubscribed();
        this.viewPrefSubscription = services.viewPreferences.getPreferences()
            .map((preferences) => preferences.appDetails.defaultKindFilter)
            .subscribe((filter) => {
                this.setState({ defaultKindFilter: filter });
            });
    }

    public componentWillUnmount() {
        this.ensureUnsubscribed();
    }

    public render() {
        return (
            <DataLoader
                ref={(loader) => this.loader = loader}
                errorRenderer={(error) => <Page title='Application Details'>{error}</Page>}
                loadingRenderer={() => <Page title='Application Details'>Loading...</Page>}
                dataChanges={this.appUpdates}
                input={this.props.match.params.name}
                load={(name) => services.applications.get(name, false)}>

                {(application: appModels.Application) => {
                    const kindsSet = new Set<string>();
                    const toProcess: (appModels.ResourceNode | appModels.ResourceState)[] = [...application.status.comparisonResult.resources || []];
                    while (toProcess.length > 0) {
                        const next = toProcess.pop();
                        const {resourceNode} = AppUtils.getStateAndNode(next);
                        kindsSet.add(resourceNode.state.kind);
                        (resourceNode.children || []).forEach((child) => toProcess.push(child));
                    }
                    const kinds = Array.from(kindsSet);
                    const kindsFilter = this.getKindsFilter().filter((kind) => kinds.indexOf(kind) > -1);
                    const filter: TopBarFilter<string> = {
                        items: kinds.map((kind) => ({ value: kind, label: kind })),
                        selectedValues: kindsFilter,
                        selectionChanged: (items) => {
                            this.appContext.apis.navigation.goto('.', { kinds: `${items.join(',')}`});
                            services.viewPreferences.updatePreferences({
                                appDetails: {
                                    defaultKindFilter: items,
                                },
                            });
                        },
                    };

                    const appNodesByName = this.groupAppNodesByName(application);
                    const selectedItem = this.selectedNodeFullName && appNodesByName.get(this.selectedNodeFullName) || null;
                    const isAppSelected = selectedItem === application;
                    const selectedNode = !isAppSelected && selectedItem as appModels.ResourceNode | appModels.ResourceState || null;
                    const operationState = application.status.operationState;
                    const conditions = application.status.conditions || [];
                    return (
                        <Page
                            title='Application Details'
                            toolbar={{ filter, breadcrumbs: [{title: 'Applications', path: '/applications' }, { title: this.props.match.params.name }], actionMenu: {
                                items: [{
                                    iconClassName: 'icon fa fa-refresh',
                                    title: 'Refresh',
                                    action: async () => {
                                        try {
                                            this.setState({ refreshing: true });
                                            await this.loader.setData(await services.applications.get(this.props.match.params.name, true));
                                        } finally {
                                            this.setState({ refreshing: false });
                                        }
                                    },
                                }, {
                                    iconClassName: 'icon argo-icon-deploy',
                                    title: 'Sync',
                                    action: () => this.setDeployPanelVisible(true),
                                }, {
                                    iconClassName: 'icon fa fa-history',
                                    title: 'History',
                                    action: () => this.setRollbackPanelVisible(0),
                                }, {
                                    iconClassName: 'icon fa fa-times-circle',
                                    title: 'Delete',
                                    action: () => this.deleteApplication(),
                                }],
                            }}}>
                            <ApplicationStatusPanel application={application}
                                showOperation={() => this.setOperationStatusVisible(true)}
                                showConditions={() => this.setConditionsStatusVisible(true)}/>
                            <div className='application-details'>
                                {this.state.refreshing && <p className='application-details__refreshing-label'>Refreshing</p>}
                                <ApplicationResourcesTree
                                    kindsFilter={kindsFilter}
                                    selectedNodeFullName={this.selectedNodeFullName}
                                    onNodeClick={(fullName) => this.selectNode(fullName)}
                                    nodeMenuItems={(node) => this.getResourceMenuItems(node)}
                                    nodeLabels={(node) => this.getResourceLabels(node)}
                                    app={application}/>
                            </div>
                            <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => this.selectNode('')}>
                                <div>
                                {selectedNode && <Tabs
                                    navTransparent={true}
                                    tabs={this.getResourceTabs(
                                        application, selectedNode, [{title: 'SUMMARY', key: 'summary', content: <ApplicationNodeInfo node={selectedNode}/>}])} />
                                }
                                {isAppSelected && (
                                    <Tabs navTransparent={true} tabs={[{
                                        title: 'SUMMARY', key: 'summary', content: <ApplicationSummary app={application} updateApp={(app) => this.updateApp(app)}/>,
                                    }, {
                                        title: 'PARAMETERS', key: 'parameters', content: <ParametersPanel
                                            updateApp={(app) => this.updateApp(app)}
                                            app={application}/>,
                                    }, {
                                        title: 'EVENTS', key: 'event', content: <ApplicationResourceEvents applicationName={application.metadata.name}/>,
                                    }]}/>
                                )}
                                </div>
                            </SlidingPanel>
                            <SlidingPanel isNarrow={true} isShown={this.showDeployPanel} onClose={() => this.setDeployPanelVisible(false)} header={(
                                    <div>
                                    <button className='argo-button argo-button--base' onClick={() => this.formApi.submitForm(null)}>
                                        Synchronize
                                    </button> <button onClick={() => this.setDeployPanelVisible(false)} className='argo-button argo-button--base-o'>
                                        Cancel
                                    </button>
                                    </div>
                                )}>
                                {this.showDeployPanel && (
                                    <Form
                                        defaultValues={{ revision: application.spec.source.targetRevision || 'HEAD'}}
                                        onSubmit={(params: any) => this.syncApplication(params.revision, params.prune)} getApi={(api) => this.formApi = api}>

                                        {(formApi) => (
                                            <form role='form' className='width-control' onSubmit={formApi.submitForm}>
                                                <h6>Synchronizing application manifests from <a href={application.spec.source.repoURL}>
                                                    {application.spec.source.repoURL}</a>
                                                </h6>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Revision' field='revision' component={Text}/>
                                                </div>
                                                <div className='argo-form-row'>
                                                    <label htmlFor='prune-on-sync-checkbox'>Prune</label> <Checkbox id='prune-on-sync-checkbox' field='prune'/>
                                                </div>
                                            </form>
                                        )}
                                    </Form>
                                )}
                            </SlidingPanel>
                            <SlidingPanel isShown={this.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                                {<ApplicationDeploymentHistory
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
                    );
                }}
            </DataLoader>
        );
    }

    private async updateApp(app: appModels.Application) {
        try {
            const updatedApp = await services.applications.update(app);
            this.loader.setData(updatedApp);
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to update application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private groupAppNodesByName(application: appModels.Application) {
        const nodeByFullName = new Map<string, appModels.ResourceState | appModels.ResourceNode | appModels.Application>();
        function addChildren<T extends (appModels.ResourceNode | appModels.ResourceState) & { fullName: string, children: appModels.ResourceNode[] }>(node: T) {
            nodeByFullName.set(node.fullName, node);
            for (const child of (node.children || [])) {
                addChildren({...child, fullName: `${child.state.kind}:${child.state.metadata.name}`});
            }
        }

        if (application) {
            nodeByFullName.set(`${application.kind}:${application.metadata.name}`, application);
            for (const node of (application.status.comparisonResult.resources || [])) {
                const state = node.liveState || node.targetState;
                addChildren({...node, children: node.childLiveResources, fullName: `${state.kind}:${state.metadata.name}`});
            }
        }
        return nodeByFullName;
    }

    private getKindsFilter() {
        let kinds = new URLSearchParams(this.props.history.location.search).get('kinds');
        if (kinds === null) {
            kinds = this.state.defaultKindFilter.join(',');
        }
        return kinds.split(',').filter((item) => !!item);
    }

    private setDeployPanelVisible(isVisible: boolean) {
        this.appContext.apis.navigation.goto('.', { deploy: isVisible });
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

    private selectNode(fullName: string, containerIndex = 0) {
        const node = fullName ? `${fullName}:${containerIndex}` : null;
        this.appContext.apis.navigation.goto('.', { node });
    }

    private async syncApplication(revision: string, prune: boolean) {
        await AppUtils.syncApplication(this.props.match.params.name, revision, prune, this.appContext);
        this.setDeployPanelVisible(false);
    }

    private async rollbackApplication(deploymentInfo: appModels.DeploymentInfo) {
        try {
            const confirmed = await this.appContext.apis.popup.confirm('Rollback application', `Are you sure you want to rollback application '${this.props.match.params.name}'?`);
            if (confirmed) {
                await services.applications.rollback(this.props.match.params.name, deploymentInfo.id);
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

    private getResourceMenuItems(resource: appModels.ResourceNode | appModels.ResourceState): MenuItem[] {
        const {resourceNode} = AppUtils.getStateAndNode(resource);
        const menuItems: {title: string, action: () => any}[] = [{
            title: 'Details',
            action: () => this.selectNode(`${resourceNode.state.kind}:${resourceNode.state.metadata.name}`),
        }];

        if (resourceNode.state.kind !== 'Application') {
            menuItems.push({
                title: 'Delete',
                action: async () => {
                    const confirmed = await this.appContext.apis.popup.confirm('Delete resource',
                        `Are your sure you want to delete ${resourceNode.state.kind} '${resourceNode.state.metadata.name}'?`);
                    if (confirmed) {
                        this.deleteResource(resourceNode.state.metadata.name, resourceNode.state.apiVersion, resourceNode.state.kind);
                    }
                },
            });
        }
        return menuItems;
    }

    private async deleteResource(resourceName: string, apiVersion: string, kind: string) {
        try {
            await services.applications.deleteResource(this.props.match.params.name, resourceName, apiVersion, kind);
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to delete pod' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private async deleteApplication() {
        await AppUtils.deleteApplication(this.props.match.params.name, this.appContext);
        this.appContext.apis.navigation.goto('/applications');
    }

    private getResourceLabels(resource: appModels.ResourceNode | appModels.ResourceState): string[] {
        const labels: string[] = [];
        const {resourceNode} = AppUtils.getStateAndNode(resource);
        switch (resourceNode.state.kind) {
            case 'Pod':
                const {reason}  = AppUtils.getPodStateReason(resourceNode.state);
                if (reason) {
                    labels.push(reason);
                }

                const allStatuses = resourceNode.state.status && resourceNode.state.status.containerStatuses || [];
                const readyStatuses = allStatuses.filter((s: ContainerStatus) => ('running' in s.state));
                const readyContainers = `${readyStatuses.length}/${allStatuses.length} containers ready`;
                labels.push(readyContainers);
                break;

            case 'Application':
                const appSrc = resourceNode.state.spec.source;
                const countOfOverrides = 'componentParameterOverrides' in appSrc && appSrc.componentParameterOverrides.length || 0;
                if (countOfOverrides > 0) {
                    labels.push('' + countOfOverrides + ' parameter override(s)');
                }
                break;
        }
        return labels;
    }

    private getResourceTabs(application: appModels.Application, resource: appModels.ResourceNode | appModels.ResourceState, tabs: Tab[]) {
        const {resourceNode} = AppUtils.getStateAndNode(resource);
        tabs.push({
            title: 'EVENTS', key: 'events', content: <ApplicationResourceEvents applicationName={this.props.match.params.name} resource={resourceNode}/>,
        });
        if (resourceNode.state.kind === 'Pod') {
            tabs = tabs.concat([{
                key: 'logs',
                title: 'LOGS',
                content: (
                    <div className='application-details__tab-content-full-height'>
                        <div className='row'>
                            <div className='columns small-3 medium-2'>
                                <p>CONTAINERS:</p>
                                {resourceNode.state.spec.containers.map((container: any, i: number) => (
                                    <div className='application-details__container' key={container.name} onClick={() => this.selectNode(this.selectedNodeFullName, i)}>
                                        {i === this.selectedNodeContainer.container && <i className='fa fa-angle-right'/>}
                                        <span title={container.name}>{container.name}</span>
                                    </div>
                                ))}
                            </div>
                            <div className='columns small-9 medium-10'>
                                <PodsLogsViewer
                                    pod={resourceNode.state} applicationName={application.metadata.name} containerIndex={this.selectedNodeContainer.container} />
                            </div>
                        </div>
                    </div>
                ),
            }]);
        }
        return tabs;
    }

    private ensureUnsubscribed() {
        if (this.viewPrefSubscription) {
            this.viewPrefSubscription.unsubscribe();
            this.viewPrefSubscription = null;
        }
    }
}
