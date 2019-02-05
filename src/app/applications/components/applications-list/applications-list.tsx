import { MockupList, NotificationType, SlidingPanel } from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';
import { Observable } from 'rxjs';

import { DataLoader, ErrorNotification, ObservableQuery, Page, Paginate } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { AppsListPreferences, services } from '../../../shared/services';
import { ApplicationCreationWizardContainer, NewAppParams, WizardStepState } from '../application-creation-wizard/application-creation-wizard';
import * as AppUtils from '../utils';
import { ApplicationsFilter } from './applications-filter';
import { ApplicationsTable } from './applications-table';
import { ApplicationTiles } from './applications-tiles';

require('./applications-list.scss');

const APP_FIELDS = ['metadata.name', 'metadata.resourceVersion', 'metadata.creationTimestamp', 'spec', 'status.sync.status', 'status.health'];
const APP_LIST_FIELDS = APP_FIELDS.map((field) => `items.${field}`);
const APP_WATCH_FIELDS = ['result.type', ...APP_FIELDS.map((field) => `result.application.${field}`)];

function loadApplications(): Observable<models.Application[]> {
    return Observable.fromPromise(services.applications.list([], { fields: APP_LIST_FIELDS })).flatMap((applications) =>
        Observable.merge(
            Observable.from([applications]),
            services.applications.watch(null, { fields: APP_WATCH_FIELDS }).map((appChange) => {
                const index = applications.findIndex((item) => item.metadata.name === appChange.application.metadata.name);
                if (index > -1 && appChange.application.metadata.resourceVersion === applications[index].metadata.resourceVersion) {
                    return {applications, updated: false};
                }
                switch (appChange.type) {
                    case 'DELETED':
                        if (index > -1) {
                            applications.splice(index, 1);
                        }
                        break;
                    default:
                        if (index > -1) {
                            applications[index] = appChange.application;
                        } else {
                            applications.unshift(appChange.application);
                        }
                        break;
                }
                return {applications, updated: true};
            }).filter((item) => item.updated).map((item) => item.applications),
        ),
    );
}

const ViewPref = ({children}: { children: (pref: AppsListPreferences) => React.ReactNode }) => (
    <ObservableQuery>
        {(q) => (
            <DataLoader load={() => Observable.combineLatest(
                services.viewPreferences.getPreferences().map((item) => item.appList), q).map((items) => {
                    const params = items[1];
                    const viewPref: AppsListPreferences = {...items[0]};
                    if (params.get('proj') != null) {
                        viewPref.projectsFilter = params.get('proj').split(',').filter((item) => !!item);
                    }
                    if (params.get('sync') != null) {
                        viewPref.syncFilter = params.get('sync').split(',').filter((item) => !!item);
                    }
                    if (params.get('health') != null) {
                        viewPref.healthFilter = params.get('health').split(',').filter((item) => !!item);
                    }
                    if (params.get('page') != null) {
                        viewPref.page = parseInt(params.get('page'), 10);
                    }
                    return {...viewPref};
            })}>
            {(pref) => children(pref)}
            </DataLoader>
        )}
    </ObservableQuery>
);

function filterApps(applications: models.Application[], pref: AppsListPreferences) {
    return applications.filter((app) =>
        (pref.projectsFilter.length === 0 || pref.projectsFilter.includes(app.spec.project)) &&
        (pref.reposFilter.length === 0 || pref.reposFilter.includes(app.spec.source.repoURL)) &&
        (pref.syncFilter.length === 0 || pref.syncFilter.includes(app.status.sync.status)) &&
        (pref.healthFilter.length === 0 || pref.healthFilter.includes(app.status.health.status)),
    );
}

export class ApplicationsList extends React.Component<RouteComponentProps<{}>, { appWizardStepState: WizardStepState; }> {

    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
    };

    private get showNewAppPanel() {
        return new URLSearchParams(this.props.location.search).get('new') === 'true';
    }

    constructor(props: RouteComponentProps<{}>) {
        super(props);
        this.state = { appWizardStepState: { next: null, prev: null, nextTitle: 'Next'} };
    }

    public render() {
        return (
        <Page title='Applications' toolbar={services.viewPreferences.getPreferences().map((pref) => ({
            breadcrumbs: [{ title: 'Applications', path: '/applications' }],
            tools: (
                <React.Fragment key='app-list-tools'>
                    <div className='applications-list__view-type'>
                        <i className={classNames('fa fa-th', {selected: pref.appList.view === 'tiles'})}
                            onClick={() => services.viewPreferences.updatePreferences({ appList: {...pref.appList, view: 'tiles'} })} />
                        <i className={classNames('fa fa-table', {selected: pref.appList.view === 'list'})}
                            onClick={() => services.viewPreferences.updatePreferences({ appList: {...pref.appList, view: 'list'} })} />
                    </div>
                </React.Fragment>
            ),
            actionMenu: {
                className: 'fa fa-plus',
                items: [{
                    title: 'New Application',
                    action: () => this.setNewAppPanelVisible(true),
                }],
            },
        }))}>
            <div className='applications-list'>
                <DataLoader
                    load={() => loadApplications()}
                    loadingRenderer={() => (<div className='argo-container'><MockupList height={100} marginTop={30}/></div>)}>
                    {(applications: models.Application[]) => (
                        applications.length === 0 ? (
                            <div className='argo-container applications-list__empty-state'>
                                <div className='applications-list__empty-state-icon'>
                                    <i className='argo-icon argo-icon-application'/>
                                </div>
                                <h4>No applications yet</h4>
                                <h5>Create new application to start managing resources in your cluster</h5>
                                <button className='argo-button argo-button--base' onClick={() => this.setNewAppPanelVisible(true)}>Create application</button>
                            </div>
                        ) : (
                            <div className='row'>
                                <div className='columns small-12 xxlarge-2'>
                                    <ViewPref>
                                    {(pref) => <ApplicationsFilter applications={applications} pref={pref} onChange={(newPref) => {
                                        services.viewPreferences.updatePreferences({appList: newPref});
                                        this.appContext.apis.navigation.goto('.', {
                                            proj: newPref.projectsFilter.join(','),
                                            sync: newPref.syncFilter.join(','),
                                            health: newPref.healthFilter.join(','),
                                        });
                                    }} />}
                                    </ViewPref>
                                </div>
                                <div className='columns small-12 xxlarge-10'>
                                <ViewPref>
                                {(pref) => (
                                    <Paginate
                                        page={pref.page}
                                        pageLimit={16}
                                        emptyState={() => (
                                            <div className='argo-container applications-list__empty-state'>
                                                <div className='applications-list__empty-state-icon'>
                                                    <i className='argo-icon argo-icon-search'/>
                                                </div>
                                                <h4>No applications found</h4>
                                                <h5>Try to change filter criteria</h5>
                                            </div>
                                        )}
                                        data={filterApps(applications, pref)} onPageChange={(page) => this.appContext.apis.navigation.goto('.', { page })} >
                                    {(data) => (
                                        pref.view === 'tiles' && (
                                            <ApplicationTiles
                                                applications={data}
                                                syncApplication={(appName, revision) => this.syncApplication.bind(appName, revision)}
                                                deleteApplication={(appName) => this.deleteApplication(appName)}
                                            />
                                        ) || (
                                            <ApplicationsTable applications={data} />
                                        )
                                    )}
                                    </Paginate>
                                )}
                                </ViewPref>
                                </div>
                            </div>
                        )
                    )}
                </DataLoader>
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

    private async createApplication(params: NewAppParams) {
        try {
            const source = {
                path: params.path,
                repoURL: params.repoURL,
                targetRevision: params.revision,
                componentParameterOverrides: null,
            } as models.ApplicationSource;
            if (params.valueFiles) {
                source.helm = {
                    valueFiles: params.valueFiles,
                } as models.ApplicationSourceHelm;
            }
            if (params.environment) {
                source.ksonnet = {
                    environment: params.environment,
                } as models.ApplicationSourceKsonnet;
            }
            if (params.namePrefix) {
                source.kustomize = {
                    namePrefix: params.namePrefix,
                } as models.ApplicationSourceKustomize;
            }
            await services.applications.create(params.applicationName, params.project, source, {
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
        const synced = await AppUtils.syncApplication(appName, revision, false, false, null, this.appContext);
        if (synced) {
            this.appContext.apis.notifications.show({
                type: NotificationType.Success,
                content: `Synced revision`,
            });
        }
    }

    private async deleteApplication(appName: string) {
        const deleted = await AppUtils.deleteApplication(appName, this.appContext);
        if (deleted) {
            this.appContext.router.history.push('/applications');
        }
    }
}
