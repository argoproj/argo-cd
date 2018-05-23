import { MockupList, NotificationType, SlidingPanel } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import { Page } from '../../../shared/components';
import { FormField } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import { ComparisonStatusIcon, HealthStatusIcon } from '../utils';

require('./applications-list.scss');

interface NewAppParams {
    applicationName: string;
    path: string;
    repoURL: string;
    environment: string;
    clusterURL: string;
    namespace: string;
}

export class ApplicationsList extends React.Component<RouteComponentProps<{}>, { applications: models.Application[] }> {

    public static contextTypes = {
        router: PropTypes.object,
        notificationManager: PropTypes.object,
    };

    private formApi: FormApi;
    private changesSubscription: Subscription;

    private get showNewAppPanel() {
        return new URLSearchParams(this.props.location.search).get('new') === 'true';
    }

    constructor(props: RouteComponentProps<{}>) {
        super(props);
        this.state = { applications: [] };
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
                                    <div className='row' onClick={() => this.appContext.router.history.push(`/applications/${app.metadata.namespace}/${app.metadata.name}`)}>
                                        <div className='columns small-3'>
                                            <div className='row'>
                                                <div className='columns small-12'>
                                                    <i className='argo-icon-application icon'/> <span className='applications-list__title'>{app.metadata.name}</span>
                                                </div>
                                            </div>
                                            <div className='row applications-list__icons'>
                                                <div className='columns small-6'>STATUS:</div>
                                                <div className='columns small-6'>
                                                    <ComparisonStatusIcon status={app.status.comparisonResult.status} /> {app.status.comparisonResult.status}
                                                </div>
                                            </div>
                                            <div className='row applications-list__icons'>
                                                <div className='columns small-6'>HEALTH:</div>
                                                <div className='columns small-6'>
                                                    <HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                                                </div>
                                            </div>
                                        </div>
                                        <div className='columns small-9 applications-list__info'>
                                            <div className='row'>
                                                <div className='columns small-3'>CLUSTER:</div>
                                                <div className='columns small-9'>{app.spec.destination.server}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>NAMESPACE:</div>
                                                <div className='columns small-9'>{app.spec.destination.namespace}</div>
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
                <SlidingPanel isShown={this.showNewAppPanel} onClose={() => this.setNewAppPanelVisible(false)} isMiddle={true} header={(
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.formApi.submitForm(null)}>
                                Create
                            </button> <button onClick={() => this.setNewAppPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    <Form onSubmit={(params) => this.createApplication(params as NewAppParams)}
                        getApi={(api) => this.formApi = api}
                        validateError={(params: NewAppParams) => ({
                            applicationName: !params.applicationName && 'Application name is required',
                            repoURL: !params.repoURL && 'Repository URL is required',
                            path: !params.path && 'Path is required',
                            environment: !params.environment && 'Environment is required',
                            clusterURL: !params.clusterURL && 'Cluster URL is required',
                            namespace: !params.namespace && 'Namespace URL is required',
                        })}>
                        {(formApi) => (
                            <form onSubmit={formApi.submitForm} role='form' className='width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Application Name' field='applicationName' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Repository URL' field='repoURL' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Path' field='path' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Environment' field='environment' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Cluster URL' field='clusterURL' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Namespace' field='namespace' component={Text}/>
                                </div>
                            </form>
                        )}
                    </Form>
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
            this.appContext.notificationManager.showNotification({
                type: NotificationType.Error,
                content: `Unable to create application: ${e.response && e.response.text || 'Internal error'}`,
            });
        }
    }
}
