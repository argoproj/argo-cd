import { AppContext, AppState, models, Page, SlidingPanel } from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import * as appModels from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';
import { ApplicationResourceDiff } from '../application-resource-diff/application-resource-diff';
import { ComparisonStatusIcon } from '../utils';

require('./application-details.scss');

export interface ApplicationDetailsProps extends RouteComponentProps<{ name: string; namespace: string; }> {
    application: appModels.Application;
    onLoad: (namespace: string, name: string) => any;
    sync: (namespace: string, name: string, revision: string) => any;
    changesSubscription: Subscription;
    showDeployPanel: boolean;
}

class Component extends React.Component<ApplicationDetailsProps, { expandedRows: number[], deployRevision: string }> {

    public static contextTypes = {
        router: PropTypes.object,
    };

    constructor(props: ApplicationDetailsProps) {
        super(props);
        this.state = { expandedRows: [], deployRevision: props.application && props.application.spec.source.targetRevision };
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

    public render() {
        return (
            <Page
                title={'Application Details'}
                toolbar={{breadcrumbs: [{title: 'Applications', path: '/applications' }, { title: this.props.match.params.name }], actionMenu: {
                    items: [{
                        className: 'icon argo-icon-deploy',
                        title: 'Deploy',
                        action: () => this.setDeployPanelVisible(true),
                    }],
                } }}>
                <div className='argo-container application-details'>
                    {this.props.application ? (
                        <div>
                            {this.renderSummary(this.props.application)}
                            {this.renderAppResouces(this.props.application)}
                            {this.props.application.status.recentDeployment && this.renderDeploymentStatus(this.props.application.status.recentDeployment)}
                        </div>
                    ) : (
                        <div>Loading...</div>
                    )}
                </div>
                <SlidingPanel isNarrow={true} isShown={this.props.showDeployPanel} onClose={() => this.setDeployPanelVisible(false)} header={(
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.syncApplication()}>
                                Deploy
                            </button> <button onClick={() => this.setDeployPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    { this.props.application && (
                        <form>
                            <h6>Deploying application manifests from <a href={this.props.application.spec.source.repoURL}>{this.props.application.spec.source.repoURL}</a></h6>
                            <h6>Revision:
                                <input className='argo-field' placeholder='latest' value={this.state.deployRevision}
                                    onChange={(event) => this.setState({ deployRevision: event.target.value })}/>
                            </h6>
                        </form>
                    )}
                </SlidingPanel>
            </Page>
        );
    }

    private renderDeploymentStatus(info: appModels.DeploymentInfo) {
        const componentParams = new Map<string, (appModels.ComponentParameter & {original: string})[]>();
        (info.params || []).map((param) => {
            const override = (info.appSource.componentParameterOverrides || []).find((item) => item.component === param.component && item.name === param.name);
            const res = {...param, original: ''};
            if (override) {
                res.original = res.value;
                res.value = override.value;
            }
            return res;
        }).forEach((param) => {
            const params = componentParams.get(param.component) || [];
            params.push(param);
            componentParams.set(param.component, params);
        });
        return (
            <div>
                <h6>Deployment parameters:</h6>
                <div className='white-box'>
                    <div className='white-box__details'>
                    {Array.from(componentParams.keys()).map((component) => (
                        componentParams.get(component).map((param, i) => (
                            <div className='row white-box__details-row' key={component + param.name}>
                                <div className='columns small-2'>
                                    {i === 0 && component.toUpperCase()}
                                </div>
                                <div className='columns small-2'>
                                    {param.name}:
                                </div>
                                <div className='columns small-8 application-details__param'>
                                    <span title={param.value}>
                                        {param.original && <span className='fa fa-exclamation-triangle' title={`Original value: ${param.original}`}/>}
                                        {param.value}
                                    </span>
                                </div>
                            </div>
                        ))
                    ))}
                    </div>
                </div>
            </div>
        );
    }

    private renderAppResouces(app: appModels.Application) {
        const resources = (app.status.comparisonResult.resources || []).map((resource) => ({
            liveState: JSON.parse(resource.liveState) as models.TypeMeta & { metadata: models.ObjectMeta },
            targetState: JSON.parse(resource.targetState) as models.TypeMeta & { metadata: models.ObjectMeta },
            status: resource.status,
        }));
        return (
            <div>
                <h6>Resources:</h6>
                <div className='argo-table-list argo-table-list--clickable'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-3'>Kind</div>
                        <div className='columns small-2'>API Version</div>
                        <div className='columns small-4'>Name</div>
                        <div className='columns small-3'>Status</div>
                    </div>
                </div>
                    {resources.map((resource, i) => (
                        <div className='argo-table-list__row' key={i} onClick={() => this.toggleRow(i)}>
                            <div className='row'>
                                <div className='columns small-3'>
                                    <i className='fa fa-gear'/> {resource.targetState.kind}
                                </div>
                                <div className='columns small-2'>{resource.targetState.apiVersion}</div>
                                <div className='columns small-4'>{resource.targetState.metadata.name}</div>
                                <div className='columns small-3'>
                                    <ComparisonStatusIcon status={resource.status}/> {resource.status}
                                    <i className={classNames('fa application-details__toggle-manifest', {
                                        'fa-angle-down': this.state.expandedRows.indexOf(i) === -1,
                                        'fa-angle-up': this.state.expandedRows.indexOf(i) !== -1})}/>
                                </div>
                            </div>
                            <div className={classNames('application-details__manifest', {'application-details__manifest--expanded': this.state.expandedRows.includes(i)})}>
                                <ApplicationResourceDiff key={i} targetState={resource.targetState} liveState={resource.liveState}/>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
        );
    }

    private renderSummary(app: appModels.Application) {
        const attributes = [
            {title: 'CLUSTER', value: app.status.comparisonResult.server},
            {title: 'NAMESPACE', value: app.status.comparisonResult.namespace},
            {title: 'REPO URL', value: (
                <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                    <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                </a> )},
            {title: 'PATH', value: app.spec.source.path},
            {title: 'ENVIRONMENT', value: app.spec.source.environment},
            {title: 'STATUS', value: (
                <span><ComparisonStatusIcon status={app.status.comparisonResult.status}/> {app.status.comparisonResult.status}</span>
            )},
        ];
        if (app.status.comparisonResult.error) {
            attributes.push({title: 'COMPARISON ERROR', value: app.status.comparisonResult.error});
        }
        return (
            <div className='white-box'>
                <div className='white-box__details'>
                    {attributes.map((attr) => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>
                                {attr.title}
                            </div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
        );
    }

    private syncApplication() {
        this.props.sync(this.props.application.metadata.namespace, this.props.application.metadata.name, this.state.deployRevision);
        this.setDeployPanelVisible(false);
    }

    private toggleRow(row: number) {
        const rows = this.state.expandedRows.slice();
        const index = rows.indexOf(row);
        if (index > -1) {
            rows.splice(index, 1);
        } else {
            rows.push(row);
        }
        this.setState({ expandedRows: rows });
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}

export const ApplicationDetails = connect((state: AppState<State>) => ({
    application: state.page.application,
    changesSubscription: state.page.changesSubscription,
    showDeployPanel: new URLSearchParams(state.router.location.search).get('deploy') === 'true',
}), (dispatch) => ({
    onLoad: (namespace: string, name: string) => dispatch(actions.loadApplication(name)),
    sync: (namespace: string, name: string, revision: string) => dispatch(actions.syncApplication(name, revision)),
}))(Component);
