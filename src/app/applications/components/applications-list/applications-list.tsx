import { DropDownMenu, MockupList, NotificationType, SlidingPanel, TopBarFilter } from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';
import { Subscription } from 'rxjs';

import { ErrorNotification, Page } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import { ApplicationCreationWizardContainer, NewAppParams, WizardStepState } from '../application-creation-wizard/application-creation-wizard';
import { ComparisonStatusIcon, HealthStatusIcon } from '../utils';

require('./applications-list.scss');

type Props = RouteComponentProps<{}>;
interface State { allProjects: string[]; projectsFilter: string[]; applications: models.Application[]; appWizardStepState: WizardStepState; }

export class ApplicationsList extends React.Component<Props, State> {

    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
    };

    public static getDerivedStateFromProps(props: Props, state: State): Partial<State> {
        const projs = new URLSearchParams(props.history.location.search).getAll('proj');
        if (projs.join(',') !== state.projectsFilter.join(',')) {
            return {
                applications: null,
                projectsFilter: projs,
            };
        }
        return null;
    }

    private changesSubscription: Subscription;

    private get showNewAppPanel() {
        return new URLSearchParams(this.props.location.search).get('new') === 'true';
    }

    constructor(props: Props) {
        super(props);
        this.state = { allProjects: [], applications: null, appWizardStepState: { next: null, prev: null, nextTitle: 'Next'}, projectsFilter: [] };
    }

    public async componentDidUpdate(prevProps: Props, prevState: State) {
        if (this.state.applications === null) {
            this.setState({ applications: (await services.applications.list(this.state.projectsFilter)) });
        }
    }

    public async componentDidMount() {
        this.setState({ applications: (await services.applications.list(this.state.projectsFilter)) });
        this.ensureUnsubscribed();
        this.changesSubscription = services.applications.watch().subscribe((applicationChange) => {
            if (this.state.applications) {
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
            }
        });
        this.setState({ allProjects: await services.projects.list().then((items) => items.map((item) => item.metadata.name)) });
    }

    public componentWillUnmount() {
        this.ensureUnsubscribed();
    }

    public render() {
        const projectsFilter = new URLSearchParams(this.props.history.location.search).getAll('proj');
        const filter: TopBarFilter<string> = {
            items: this.state.allProjects.map((proj) => ({ value: proj, label: proj })),
            selectedValues: projectsFilter,
            selectionChanged: (items) => {
                this.appContext.apis.navigation.goto('.', { proj: items});
            },
        };
        return (
        <Page title='Applications' toolbar={{
                filter,
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
                        <div className='argo-table-list argo-table-list--clickable row small-up-1 large-up-2'>
                            {this.state.applications.map((app) => (
                                <div key={app.metadata.name} className='column column-block'>
                                    <div className={`argo-table-list__row
                                        applications-list__entry applications-list__entry--comparison-${app.status.comparisonResult.status}`
                                    }>
                                        <div className='row' onClick={() => this.appContext.router.history.push(`/applications/${app.metadata.name}`)}>
                                            <div className='columns small-12 applications-list__info'>
                                                <div className='row'>
                                                    <div className='columns applications-list__title'>{app.metadata.name}</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Project:</div>
                                                    <div className='columns small-9'>{app.spec.project}</div>
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
                                                    <div className='columns small-3'>Status:</div>
                                                    <div className='columns small-9'>
                                                        <ComparisonStatusIcon status={app.status.comparisonResult.status}/> {app.status.comparisonResult.status}
                                                    </div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Health:</div>
                                                    <div className='columns small-9'>
                                                        <HealthStatusIcon state={app.status.health}/> {app.status.health.status}
                                                    </div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Age:</div>
                                                    <div className='columns small-9'>{this.daysBeforeNow(app.metadata.creationTimestamp)} days</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Repository:</div>
                                                    <div className='columns small-9'>
                                                        <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                                                            <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                                                        </a>
                                                    </div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Path:</div>
                                                    <div className='columns small-9'>{app.spec.source.path}</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns small-3'>Environment:</div>
                                                    <div className='columns small-9'>{app.spec.source.environment}</div>
                                                </div>
                                                <div className='row'>
                                                    <div className='columns applications-list__entry--actions'>
                                                        <DropDownMenu anchor={() =>
                                                            <button className='argo-button argo-button--base-o'>Actions  <i className='fa fa-caret-down'/></button>
                                                        } items={[
                                                            { title: 'Sync', action: () => this.syncApplication(app.metadata.name, 'HEAD') },
                                                            { title: 'Delete', action: () => this.deleteApplication(app.metadata.name, false) },
                                                        ]} />
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : <MockupList height={300} marginTop={30}/>}
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
                    {this.showNewAppPanel && <ApplicationCreationWizardContainer
                        createApp={(params) => this.createApplication(params)}
                        onStateChanged={(appWizardStepState) => this.setState({ appWizardStepState })} />
                    }
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

    // DaysBeforeNow returns the delta, in days, between now and a given timestamp.
    private daysBeforeNow(timestamp: string): number {
        const end = moment();
        const start = moment(timestamp);
        const delta = moment.duration(end.diff(start));
        return Math.round(delta.asDays());
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
                content: <ErrorNotification title='Unable to create application' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private async syncApplication(appName: string, revision: string) {
        try {
            await services.applications.sync(appName, revision, false).then(() => {
                this.appContext.apis.notifications.show({
                    type: NotificationType.Success,
                    content: `Synced revision`,
                });
            });
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to deploy revision' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private async deleteApplication(appName: string, force: boolean) {
        const confirmed = await this.appContext.apis.popup.confirm('Delete application', `Are your sure you want to delete application '${appName}'?`);
        if (confirmed) {
            try {
                await services.applications.delete(appName, force);
                this.appContext.router.history.push('/applications');
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to delete application' e={e}/>,
                    type: NotificationType.Error,
                });
            }
        }
    }
}
