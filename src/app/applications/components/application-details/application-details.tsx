import { AppContext, AppState, MenuItem, Page, SlidingPanel, Tab, Tabs} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import * as appModels from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';

import { ApplicationNodeInfo } from '../application-node-info/application-node-info';
import { ApplicationResourcesTree } from '../application-resources-tree/application-resources-tree';
import { ApplicationSummary } from '../application-summary/application-summary';
import { ParametersPanel } from '../parameters-panel/parameters-panel';
import { PodsLogsViewer } from '../pod-logs-viewer/pod-logs-viewer';
import * as AppUtils from '../utils';

require('./application-details.scss');

export interface ApplicationDetailsProps extends RouteComponentProps<{ name: string; namespace: string; }> {
    application: appModels.Application;
    onLoad: (namespace: string, name: string) => any;
    sync: (namespace: string, name: string, revision: string) => any;
    deletePod: (namespace: string, appName: string, podName: string) => any;
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
        this.state = { deployRevision: props.application && props.application.spec.source.targetRevision };
    }

    public componentWillReceiveProps(props: ApplicationDetailsProps) {
        this.setState({deployRevision: props.application && props.application.spec.source.targetRevision});
    }

    public componentDidMount() {
        this.props.onLoad(this.props.match.params.namespace, this.props.match.params.name);
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
        const recentDeployments = this.props.application && this.props.application.status.recentDeployments || [];
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
                        title: 'Deploy',
                        action: () => this.setDeployPanelVisible(true),
                    }, ...(recentDeployments.length > 1 ? [{
                        className: 'icon fa fa-undo',
                        title: 'Rollback',
                        action: () => this.setRollbackPanelVisible(0),
                    }] : [])],
                } }}>
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
                            <button className='argo-button argo-button--base' onClick={() => this.syncApplication()}>
                                Deploy
                            </button> <button onClick={() => this.setDeployPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    {this.props.application && (
                        <form>
                            <h6>Deploying application manifests from <a href={this.props.application.spec.source.repoURL}>{this.props.application.spec.source.repoURL}</a></h6>
                            <h6>Revision:
                                <input className='argo-field' placeholder='latest' value={this.state.deployRevision}
                                    onChange={(event) => this.setState({ deployRevision: event.target.value })}/>
                            </h6>
                        </form>
                    )}
                </SlidingPanel>
                <SlidingPanel isShown={this.props.selectedRollbackDeploymentIndex > -1} onClose={() => this.setRollbackPanelVisible(-1)}>
                    <div className='row'>
                        <div className='columns small-3'>
                            {recentDeployments.slice(1).map((info, i) => (
                                <p key={i}>
                                    {this.props.selectedRollbackDeploymentIndex === i ? <span>{info.revision}</span> : (
                                        <a onClick={() => this.setRollbackPanelVisible(i)}>{info.revision}</a>
                                    )}
                                </p>
                            ))}
                        </div>
                        <div className='columns small-9'>
                            {recentDeployments[this.props.selectedRollbackDeploymentIndex] && (
                                <ParametersPanel
                                    params={recentDeployments[this.props.selectedRollbackDeploymentIndex].params}
                                    overrides={recentDeployments[this.props.selectedRollbackDeploymentIndex].componentParameterOverrides}/>
                            )}
                            <br/>
                            <button className='argo-button argo-button--base' onClick={() => this.syncApplication()}>
                                Rollback
                            </button> <button onClick={() => this.setRollbackPanelVisible(-1)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    </div>
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

    private syncApplication(revision: string = null) {
        this.props.sync(this.props.application.metadata.namespace, this.props.application.metadata.name, revision || this.state.deployRevision);
        this.setDeployPanelVisible(false);
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
                action: () => this.props.deletePod(this.props.match.params.namespace, this.props.match.params.name, resourceNode.state.metadata.name),
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
    onLoad: (namespace: string, name: string) => dispatch(actions.loadApplication(name)),
    sync: (namespace: string, name: string, revision: string) => dispatch(actions.syncApplication(name, revision)),
    deletePod: (namespace: string, appName: string, podName: string) => dispatch(actions.deletePod(appName, podName)),
}))(Component);
