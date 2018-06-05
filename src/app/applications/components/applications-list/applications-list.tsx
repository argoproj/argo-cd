import { MockupList, NotificationType, SlidingPanel } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import { Page } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import { ApplicationCreationWizardContainer, NewAppParams, WizardStepState } from '../application-creation-wizard/application-creation-wizard';

require('./applications-list.scss');

export class ApplicationsList extends React.Component<RouteComponentProps<{}>, { applications: models.Application[], appWizardStepState: WizardStepState }> {

    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
    };

    private changesSubscription: Subscription;

    private get showNewAppPanel() {
        return new URLSearchParams(this.props.location.search).get('new') === 'true';
    }

    constructor(props: RouteComponentProps<{}>) {
        super(props);
        this.state = { applications: [], appWizardStepState: { next: null, prev: null, nextTitle: 'Next'} };
    }

    public async componentDidMount() {
        this.ensureUnsubscribed();
        this.setState({ applications: (await services.applications.list()) });
        this.changesSubscription = services.applications.watch().subscribe((applicationChange) => {
            const applications = this.state.applications.slice();
            switch (applicationChange.type) {
                case 'ADDED':
                case 'MODIFIED':
                    const index = applications.findIndex((item) => item.metadata.name === applicationChange.application.metadata.name);
                    if (index > -1) {
                        applications[index] = applicationChange.application;
                        this.setState({ applications });
                    } else {
                        applications.unshift(applicationChange.application);
                        this.setState({ applications });
                    }
                    break;
                case 'DELETED':
                    this.setState({ applications: applications.filter((item) => item.metadata.name !== applicationChange.application.metadata.name) });
                    break;
            }
        });
    }

    public componentWillUnmount() {
        this.ensureUnsubscribed();
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
                    {this.state.applications ? (
                        <div className='argo-table-list argo-table-list--clickable'>
                            {this.state.applications.map((app) => (
                                <div key={app.metadata.name} className='argo-table-list__row'>
                                    <div className={`row applications-list__entry applications-list__entry--health-${app.status.health.status}`} onClick={() => this.appContext.router.history.push(`/applications/${app.metadata.namespace}/${app.metadata.name}`)}>
                                        <div className='columns small-12 applications-list__info'>
                                            <div className='row'>
                                                <div className='columns applications-list__title'>{app.metadata.name}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>Namespace:</div>
                                                <div className='columns small-9'>{app.spec.destination.namespace}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>Cluster:</div>
                                                <div className='columns small-9'>{app.spec.destination.server}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>k8sVersion:</div>
                                                <div className='columns small-9'>{app.spec.destination.server}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>Status:</div>
                                                <div className='columns small-9'>{app.status.comparisonResult.status}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>Age:</div>
                                                <div className='columns small-9'>{app.status.comparisonResult.status}</div>
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
                <SlidingPanel isShown={this.showNewAppPanel} onClose={() => this.setNewAppPanelVisible(false)} header={
                        <div>
                            <button className='argo-button argo-button--base'
                                    disabled={!this.state.appWizardStepState.prev}
                                    onClick={() => this.state.appWizardStepState.prev && this.state.appWizardStepState.prev()}>
                                Back
                            </button> <button className='argo-button argo-button--base'
                                    disabled={!this.state.appWizardStepState.next}
                                    onClick={() => this.state.appWizardStepState.next && this.state.appWizardStepState.next()}>
                                {this.state.appWizardStepState.nextTitle}
                            </button> <button onClick={() => this.setNewAppPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <ApplicationCreationWizardContainer
                        createApp={(params) => this.createApplication(params)}
                        onStateChanged={(appWizardStepState) => this.setState({ appWizardStepState })} />
                </SlidingPanel>
            </Page>
        );
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private setNewAppPanelVisible(isVisible: boolean) {
        this.appContext.router.history.push(`${this.props.match.url}?new=${isVisible}`);
    }

    private ensureUnsubscribed() {
        if (this.changesSubscription) {
            this.changesSubscription.unsubscribe();
        }
        this.changesSubscription = null;
    }

    private async createApplication(params: NewAppParams) {
        try {
            await services.applications.create(params.applicationName, {
                environment: params.environment,
                path: params.path,
                repoURL: params.repoURL,
                targetRevision: null,
                componentParameterOverrides: null,
            }, {
                server: params.clusterURL,
                namespace: params.namespace,
            });
            this.setNewAppPanelVisible(false);
        } catch (e) {
            this.appContext.apis.notifications.show({
                type: NotificationType.Error,
                content: `Unable to create application: ${e.response && e.response.text || 'Internal error'}`,
            });
        }
    }
}
