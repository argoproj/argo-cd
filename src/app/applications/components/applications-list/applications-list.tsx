import { AppContext, AppState, MockupList, Page, SlidingPanel } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import * as models from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';
import { ComparisonStatusIcon } from '../utils';

require('./applications-list.scss');

export interface ApplicationProps extends RouteComponentProps<{}> {
    onLoad: () => any;
    applications: models.Application[];
    changesSubscription: Subscription;
    showNewAppPanel: boolean;
    createApp: (appName: string, source: models.ApplicationSource) => any;
}

const DEFAULT_NEW_APP = { applicationName: '', targetRevision: 'master', componentParameterOverrides: [] as any[], path: '', repoURL: '', environment: '' };

class Component extends React.Component<ApplicationProps, models.ApplicationSource & { applicationName: string }> {

    public static contextTypes = {
        router: PropTypes.object,
    };

    constructor(props: ApplicationProps) {
        super(props);
        this.state = DEFAULT_NEW_APP;
    }

    public componentDidMount() {
        this.props.onLoad();
    }

    public componentWillUnmount() {
        if (this.props.changesSubscription) {
            this.props.changesSubscription.unsubscribe();
        }
    }

    public render() {
        return (
        <Page title='Applications' toolbar={{
                breadcrumbs: [{ title: 'Applications', path: '/applications' }],
                actionMenu: {
                    className: 'fa fa-plus',
                    items: [{
                        title: 'New Application',
                        action: () => this.setNewAppPanelVisible(true),
                    }],
                },
            }} >
                <div className='argo-container applications-list'>
                    {this.props.applications ? (
                        <div className='argo-table-list argo-table-list--clickable'>
                            {this.props.applications.map((app) => (
                                <div key={app.metadata.name} className='argo-table-list__row'>
                                    <div className='row' onClick={() => this.appContext.router.history.push(`/applications/${app.metadata.namespace}/${app.metadata.name}`)}>
                                        <div className='columns small-3'>
                                            <div className='row'>
                                                <div className='columns small-12'>
                                                    <i className='argo-icon-application icon'/> <span className='applications-list__title'>{app.metadata.name}</span>
                                                </div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-6'>STATUS:</div>
                                                <div className='columns small-6'>
                                                    <ComparisonStatusIcon status={app.status.comparisonResult.status} /> {app.status.comparisonResult.status}
                                                </div>
                                            </div>
                                        </div>
                                        <div className='columns small-9 applications-list__info'>
                                            <div className='row'>
                                                <div className='columns small-3'>CLUSTER:</div>
                                                <div className='columns small-9'>{app.status.comparisonResult.server}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>NAMESPACE:</div>
                                                <div className='columns small-9'>{app.status.comparisonResult.namespace}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>REPO URL:</div>
                                                <div className='columns small-9'>
                                                    <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                                                        <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                                                    </a>
                                                </div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>PATH:</div>
                                                <div className='columns small-9'>{app.spec.source.path}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>ENVIRONMENT:</div>
                                                <div className='columns small-9'>{app.spec.source.environment}</div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : <MockupList height={50} marginTop={30}/>}
                </div>
                <SlidingPanel isShown={this.props.showNewAppPanel} onClose={() => this.setNewAppPanelVisible(false)} isMiddle={true} header={(
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.createApplication()}>
                                Create
                            </button> <button onClick={() => this.setNewAppPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    <form>
                        <h6>Name:
                            <input className='argo-field' required={true} value={this.state.applicationName}
                                onChange={(event) => this.setState({ applicationName: event.target.value })}/>
                        </h6>
                        <h6>Repository URL:
                            <input className='argo-field' required={true} value={this.state.repoURL}
                                onChange={(event) => this.setState({ repoURL: event.target.value })}/>
                        </h6>
                        <h6>Path:
                            <input className='argo-field' required={true} value={this.state.path}
                                onChange={(event) => this.setState({ path: event.target.value })}/>
                        </h6>
                        <h6>Environment:
                            <input className='argo-field' required={true} value={this.state.environment}
                                onChange={(event) => this.setState({ environment: event.target.value })}/>
                        </h6>
                    </form>
                </SlidingPanel>
            </Page>
        );
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private setNewAppPanelVisible(isVisible: boolean) {
        if (isVisible) {
            this.setState(DEFAULT_NEW_APP);
        }
        this.appContext.router.history.push(`${this.props.match.url}?new=${isVisible}`);
    }

    private createApplication() {
        if (this.state.applicationName && this.state.environment && this.state.repoURL && this.state.path) {
            this.setNewAppPanelVisible(false);
            this.props.createApp(this.state.applicationName, {
                environment: this.state.environment,
                path: this.state.path,
                repoURL: this.state.repoURL,
                targetRevision: null,
                componentParameterOverrides: null,
            });
        }
    }
}

export const ApplicationsList = connect((state: AppState<State>) => {
    return {
        applications: state.page.applications,
        changesSubscription: state.page.changesSubscription,
        showNewAppPanel: new URLSearchParams(state.router.location.search).get('new') === 'true',
    };
}, (dispatch) => ({
    onLoad: () => dispatch(actions.loadAppsList()),
    createApp: (appName: string, source: models.ApplicationSource) => dispatch(actions.createApplication(appName, source)),
}))(Component);
