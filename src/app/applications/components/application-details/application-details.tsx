import { AppContext, AppState, MenuItem, SlidingPanel, Tab, Tabs} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import * as appModels from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';

import { Page } from '../../../shared/components';
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
    application: appModels.Application;
    onLoad: typeof actions.loadApplication;
    syncApp: typeof actions.syncApplication;
    rollbackApp: typeof actions.rollbackApplication;
    deletePod: typeof actions.deletePod;
    deleteApp: typeof actions.deleteApplication;
    changesSubscription: Subscription;
    showDeployPanel: boolean;
    selectedRollbackDeploymentIndex: number;
    selectedNodeFullName: string;
}

class Component extends React.Component<ApplicationDetailsProps, { deployRevision: string }> {

    public static contextTypes = {
        router: PropTypes.object,
    };

    constructor(props: ApplicationDetailsProps) {
        super(props);
        this.state = { deployRevision: ''};
    }

    public componentDidMount() {
        this.props.onLoad(this.props.match.params.name);
    }

    public componentWillUnmount() {
        if (this.props.changesSubscription) {
            this.props.changesSubscription.unsubscribe();
        }
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
        const selectedItem = this.props.selectedNodeFullName && appNodesByName.get(this.props.selectedNodeFullName) || null;
        const isAppSelected = this.props.application != null && selectedItem === this.props.application;
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
                        action: () => this.props.deleteApp(this.props.match.params.name, true),
                    }],
                } }}>
                {this.props.application && <ApplicationStatusPanel application={this.props.application}/>}
                <div className='argo-container application-details'>
                    {this.props.application ? (
                        <ApplicationResourcesTree
                            selectedNodeFullName={this.props.selectedNodeFullName}
                            onNodeClick={(fullName) => this.selectNode(fullName)}
                            nodeMenuItems={(node) => this.getResourceMenuItems(node)}
                            nodeLabels={(node) => this.getResourceLabels(node)}
                            app={this.props.application}/>
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
                            title: 'SUMMARY', key: 'summary', content: <ApplicationSummary app={this.props.application}/>,
                        }, {
                            title: 'PARAMETERS', key: 'parameters', content: <ParametersPanel
                                params={this.props.application.status.parameters || []}
                                overrides={this.props.application.spec.source.componentParameterOverrides}/>,
                        }]}/>
                    )}
                    </div>
                </SlidingPanel>
                <SlidingPanel isNarrow={true} isShown={this.props.showDeployPanel} onClose={() => this.setDeployPanelVisible(false)} header={(
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.syncApplication(this.state.deployRevision)}>
                                Synchronize
                            </button> <button onClick={() => this.setDeployPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    {this.props.application && (
                        <form>
                            <h6>Synchronizing application manifests from <a href={this.props.application.spec.source.repoURL}>{this.props.application.spec.source.repoURL}</a></h6>
                            <h6>Revision:
                                <input className='argo-field' placeholder='latest' value={this.state.deployRevision}
                                    onChange={(event) => this.setState({ deployRevision: event.target.value })}/>
                            </h6>
                        </form>
                    )}
                </SlidingPanel>
                <SlidingPanel isShown={this.props.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                    {this.props.application && <ApplicationDeploymentHistory
                        app={this.props.application}
                        selectedRollbackDeploymentIndex={this.props.selectedRollbackDeploymentIndex}
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

        if (this.props.application) {
            nodeByFullName.set(`${this.props.application.kind}:${this.props.application.metadata.name}`, this.props.application);
            for (const node of (this.props.application.status.comparisonResult.resources || [])) {
                const state = node.liveState || node.targetState;
                addChildren({...node, children: node.childLiveResources, fullName: `${state.kind}:${state.metadata.name}`});
            }
        }
        return nodeByFullName;
    }

    private syncApplication(revision: string) {
        this.props.syncApp(this.props.application.metadata.name, revision);
        this.setDeployPanelVisible(false);
    }

    private rollbackApplication(deploymentInfo: appModels.DeploymentInfo) {
        this.props.rollbackApp(this.props.application.metadata.name, deploymentInfo.id);
        this.setRollbackPanelVisible(-1);
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
                action: () => this.props.deletePod(this.props.match.params.name, resourceNode.state.metadata.name),
            });
        }
        return menuItems;
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
                        <PodsLogsViewer pod={resourceNode.state} applicationName={this.props.application.metadata.name} />
                    </div>
                ),
            }]);
        }
        return tabs;
    }
}

export const ApplicationDetails = connect((state: AppState<State>) => ({
    application: state.page.application,
    changesSubscription: state.page.changesSubscription,
    showDeployPanel: new URLSearchParams(state.router.location.search).get('deploy') === 'true',
    selectedRollbackDeploymentIndex: parseInt(new URLSearchParams(state.router.location.search).get('rollback'), 10),
    selectedNodeFullName: new URLSearchParams(state.router.location.search).get('node'),
}), (dispatch) => ({
    onLoad: (name: string) => dispatch(actions.loadApplication(name)),
    syncApp: (name: string, revision: string) => dispatch(actions.syncApplication(name, revision)),
    deletePod: (appName: string, podName: string) => dispatch(actions.deletePod(appName, podName)),
    deleteApp: (appName: string, force: boolean) => dispatch(actions.deleteApplication(appName, force)),
    rollbackApp: (appName: string, id: number) => dispatch(actions.rollbackApplication(appName, id)),
}))(Component);
