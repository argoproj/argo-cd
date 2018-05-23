import { MenuItem, NotificationType, SlidingPanel, Tab, Tabs} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';
import { Observable, Subscription } from 'rxjs';

import { Page } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as appModels from '../../../shared/models';
import { services } from '../../../shared/services';

import { ApplicationDeploymentHistory } from '../application-deployment-history/application-deployment-history';
import { ApplicationNodeInfo } from '../application-node-info/application-node-info';
import { ApplicationResourcesTree } from '../application-resources-tree/application-resources-tree';
import { ApplicationStatusPanel } from '../application-status-panel/application-status-panel';
import { ApplicationSummary } from '../application-summary/application-summary';
import { ParametersPanel } from '../parameters-panel/parameters-panel';
import { PodsLogsViewer } from '../pod-logs-viewer/pod-logs-viewer';
import * as AppUtils from '../utils';

require('./application-details.scss');

export interface ApplicationDetailsProps extends RouteComponentProps<{ name: string; namespace: string; }> {
}

export class ApplicationDetails extends React.Component<ApplicationDetailsProps, { deployRevision: string, application: appModels.Application }> {

    public static contextTypes = {
        router: PropTypes.object,
        notificationManager: PropTypes.object,
        apis: PropTypes.object,
    };

    private changesSubscription: Subscription;

    constructor(props: ApplicationDetailsProps) {
        super(props);
        this.state = { deployRevision: '', application: null};
    }

    private get showDeployPanel() {
        return new URLSearchParams(this.props.history.location.search).get('deploy') === 'true';
    }

    private get selectedRollbackDeploymentIndex() {
        return parseInt(new URLSearchParams(this.props.history.location.search).get('rollback'), 10);
    }

    private get selectedNodeFullName() {
        return new URLSearchParams(this.props.location.search).get('node');
    }

    public async componentDidMount() {
        const appName = this.props.match.params.name;
        const appUpdates = Observable
            .from([await services.applications.get(appName)])
            .merge(services.applications.watch({name: appName}).map((changeEvent) => changeEvent.application));
        this.ensureUnsubscribed();
        this.changesSubscription = appUpdates.subscribe((application) => {
            this.setState({ application });
        });
    }

    public componentWillUnmount() {
        this.ensureUnsubscribed();
    }

    public setDeployPanelVisible(isVisible: boolean) {
        this.appContext.router.history.push(`${this.props.match.url}?deploy=${isVisible}`);
    }

    public setRollbackPanelVisible(selectedDeploymentIndex = 0) {
        this.appContext.router.history.push(`${this.props.match.url}?rollback=${selectedDeploymentIndex}`);
    }

    public selectNode(fullName: string) {
        this.appContext.router.history.push(`${this.props.match.url}?node=${fullName}`);
    }

    public render() {
        const appNodesByName = this.groupAppNodesByName();
        const selectedItem = this.selectedNodeFullName && appNodesByName.get(this.selectedNodeFullName) || null;
        const isAppSelected = this.state.application != null && selectedItem === this.state.application;
        const selectedNode = !isAppSelected && selectedItem as appModels.ResourceNode | appModels.ResourceState || null;
        return (
            <Page
                title={'Application Details'}
                toolbar={{breadcrumbs: [{title: 'Applications', path: '/applications' }, { title: this.props.match.params.name }], actionMenu: {
                    items: [{
                        className: 'icon argo-icon-deploy',
                        title: 'Sync',
                        action: () => this.setDeployPanelVisible(true),
                    }, {
                        className: 'icon fa fa-history',
                        title: 'History',
                        action: () => this.setRollbackPanelVisible(0),
                    }, {
                        className: 'icon fa fa-times-circle',
                        title: 'Delete',
                        action: () => this.deleteApplication(true),
                    }],
                } }}>
                {this.state.application && <ApplicationStatusPanel application={this.state.application}/>}
                <div className='argo-container application-details'>
                    {this.state.application ? (
                        <ApplicationResourcesTree
                            selectedNodeFullName={this.selectedNodeFullName}
                            onNodeClick={(fullName) => this.selectNode(fullName)}
                            nodeMenuItems={(node) => this.getResourceMenuItems(node)}
                            nodeLabels={(node) => this.getResourceLabels(node)}
                            app={this.state.application}/>
                    ) : (
                        <div>Loading...</div>
                    )}
                </div>
                <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => this.selectNode('')}>
                    <div>
                    {selectedNode && <Tabs
                        navTransparent={true}
                        tabs={this.getResourceTabs(selectedNode, [{ title: 'SUMMARY', key: 'summary', content: <ApplicationNodeInfo node={selectedNode}/>}])} />
                    }
                    {isAppSelected && (
                        <Tabs navTransparent={true} tabs={[{
                            title: 'SUMMARY', key: 'summary', content: <ApplicationSummary app={this.state.application}/>,
                        }, {
                            title: 'PARAMETERS', key: 'parameters', content: <ParametersPanel
                                params={this.state.application.status.parameters || []}
                                overrides={this.state.application.spec.source.componentParameterOverrides}/>,
                        }]}/>
                    )}
                    </div>
                </SlidingPanel>
                <SlidingPanel isNarrow={true} isShown={this.showDeployPanel} onClose={() => this.setDeployPanelVisible(false)} header={(
                        <div>
                        <button className='argo-button argo-button--base' onClick={() => this.syncApplication(this.state.deployRevision)}>
                            Synchronize
                        </button> <button onClick={() => this.setDeployPanelVisible(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                        </div>
                    )}>
                    {this.state.application && (
                        <form>
                            <h6>Synchronizing application manifests from <a href={this.state.application.spec.source.repoURL}>{this.state.application.spec.source.repoURL}</a></h6>
                            <h6>Revision:
                                <input className='argo-field' placeholder='latest' value={this.state.deployRevision}
                                    onChange={(event) => this.setState({ deployRevision: event.target.value })}/>
                            </h6>
                        </form>
                    )}
                </SlidingPanel>
                <SlidingPanel isShown={this.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                    {this.state.application && <ApplicationDeploymentHistory
                        app={this.state.application}
                        selectedRollbackDeploymentIndex={this.selectedRollbackDeploymentIndex}
                        rollbackApp={(info) => this.rollbackApplication(info)}
                        selectDeployment={(i) => this.setRollbackPanelVisible(i)}
                        />}
                </SlidingPanel>
            </Page>
        );
    }

    public groupAppNodesByName() {
        const nodeByFullName = new Map<string, appModels.ResourceState | appModels.ResourceNode | appModels.Application>();
        function addChildren<T extends (appModels.ResourceNode | appModels.ResourceState) & { fullName: string, children: appModels.ResourceNode[] }>(node: T) {
            nodeByFullName.set(node.fullName, node);
            for (const child of (node.children || [])) {
                addChildren({...child, fullName: `${child.state.kind}:${child.state.metadata.name}`});
            }
        }

        if (this.state.application) {
            nodeByFullName.set(`${this.state.application.kind}:${this.state.application.metadata.name}`, this.state.application);
            for (const node of (this.state.application.status.comparisonResult.resources || [])) {
                const state = node.liveState || node.targetState;
                addChildren({...node, children: node.childLiveResources, fullName: `${state.kind}:${state.metadata.name}`});
            }
        }
        return nodeByFullName;
    }

    private async syncApplication(revision: string) {
        try {
            await services.applications.sync(this.props.match.params.name, revision);
            this.setDeployPanelVisible(false);
        } catch (e) {
            this.appContext.notificationManager.showNotification({
                type: NotificationType.Error,
                content: `Unable to deploy revision: ${e.response && e.response.text || 'Internal error'}`,
            });
        }
    }

    private async rollbackApplication(deploymentInfo: appModels.DeploymentInfo) {
        try {
            await services.applications.rollback(this.props.match.params.name, deploymentInfo.id);
            this.setRollbackPanelVisible(-1);
        } catch (e) {
            this.appContext.notificationManager.showNotification({
                type: NotificationType.Error,
                content: `Unable to rollback application: ${e.response && e.response.text || 'Internal error'}`,
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

        if (resourceNode.state.kind === 'Pod') {
            menuItems.push({
                title: 'Delete',
                action: async () => {
                    const confirmed = await this.appContext.apis.popup.confirm('Delete pod', `Are your sure you want to delete pod '${resourceNode.state.metadata.name}'?`);
                    if (confirmed) {
                        this.deletePod(resourceNode.state.metadata.name);
                    }
                },
            });
        }
        return menuItems;
    }

    private async deletePod(podName: string) {
        try {
            await services.applications.deletePod(this.props.match.params.name, podName);
        } catch (e) {
            this.appContext.notificationManager.showNotification({
                type: NotificationType.Error,
                content: `Unable to delete pod: ${e.response && e.response.text || 'Internal error'}`,
            });
        }
    }

    private async deleteApplication(force: boolean) {
        const confirmed = await this.appContext.apis.popup.confirm('Delete application', `Are your sure you want to delete application '${this.props.match.params.name}'?`);
        if (confirmed) {
            try {
                await services.applications.delete(this.props.match.params.name, force);
                this.appContext.router.history.push('/applications');
            } catch (e) {
                this.appContext.notificationManager.showNotification({
                    type: NotificationType.Error,
                    content: `Unable to delete application: ${e.response && e.response.text || 'Internal error'}`,
                });
            }
        }
    }

    private getResourceLabels(resource: appModels.ResourceNode | appModels.ResourceState): string[] {
        const labels: string[] = [];
        const {resourceNode} = AppUtils.getStateAndNode(resource);
        if (resourceNode.state.kind === 'Pod') {
            const phase = AppUtils.getPodPhase(resourceNode.state);
            if (phase) {
                labels.push(phase);
            }
        }
        return labels;
    }

    private getResourceTabs(resource: appModels.ResourceNode | appModels.ResourceState, tabs: Tab[]) {
        const {resourceNode} = AppUtils.getStateAndNode(resource);
        if (resourceNode.state.kind === 'Pod') {
            tabs = tabs.concat([{
                key: 'logs',
                title: 'LOGS',
                content: (
                    <div className='application-details__tab-content-full-height'>
                        <PodsLogsViewer pod={resourceNode.state} applicationName={this.state.application.metadata.name} />
                    </div>
                ),
            }]);
        }
        return tabs;
    }

    private ensureUnsubscribed() {
        if (this.changesSubscription) {
            this.changesSubscription.unsubscribe();
        }
        this.changesSubscription = null;
    }
}
