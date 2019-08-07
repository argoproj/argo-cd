import { ErrorNotification, MockupList, NotificationType, SlidingPanel } from 'argo-ui';
import * as classNames from 'classnames';
import * as minimatch from 'minimatch';
import * as React from 'react';
import { RouteComponentProps } from 'react-router';
import { Observable } from 'rxjs';

import { Autocomplete, ClusterCtx, DataLoader, EmptyState, ObservableQuery, Page, Paginate, Query } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';
import { AppsListPreferences, AppsListViewType, services } from '../../../shared/services';
import { ApplicationCreatePanel } from '../application-create-panel/application-create-panel';
import { ApplicationSyncPanel } from '../application-sync-panel/application-sync-panel';
import * as AppUtils from '../utils';
import { ApplicationsFilter } from './applications-filter';
import { ApplicationsSummary } from './applications-summary';
import { ApplicationsTable } from './applications-table';
import { ApplicationTiles } from './applications-tiles';

require('./applications-list.scss');

const APP_FIELDS = [
    'metadata.name', 'metadata.annotations', 'metadata.resourceVersion', 'metadata.creationTimestamp', 'spec', 'status.sync.status', 'status.health', 'status.summary'];
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

const ViewPref = ({children}: { children: (pref: AppsListPreferences & { page: number, search: string }) => React.ReactNode }) => (
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
                    if (params.get('namespace') != null) {
                        viewPref.namespacesFilter = params.get('namespace').split(',').filter((item) => !!item);
                    }
                    if (params.get('cluster') != null) {
                        viewPref.clustersFilter = params.get('cluster').split(',').filter((item) => !!item);
                    }
                    if (params.get('view') != null) {
                        viewPref.view = params.get('view') as AppsListViewType;
                    }
                    return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || ''};
            })}>
            {(pref) => children(pref)}
            </DataLoader>
        )}
    </ObservableQuery>
);

function filterApps(applications: models.Application[], pref: AppsListPreferences, search: string) {
    return applications.filter((app) =>
        (search === '' || app.metadata.name.includes(search)) &&
        (pref.projectsFilter.length === 0 || pref.projectsFilter.includes(app.spec.project)) &&
        (pref.reposFilter.length === 0 || pref.reposFilter.includes(app.spec.source.repoURL)) &&
        (pref.syncFilter.length === 0 || pref.syncFilter.includes(app.status.sync.status)) &&
        (pref.healthFilter.length === 0 || pref.healthFilter.includes(app.status.health.status)) &&
        (pref.namespacesFilter.length === 0 || pref.namespacesFilter.some((ns) => minimatch(app.spec.destination.namespace, ns))) &&
        (pref.clustersFilter.length === 0 || pref.clustersFilter.some((server) => minimatch(app.spec.destination.server, server))),
    );
}

function tryJsonParse(input: string) {
    try {
        return input && JSON.parse(input) || null;
    } catch {
        return null;
    }
}

export const ApplicationsList = (props: RouteComponentProps<{}>) => {
    const query = new URLSearchParams(props.location.search);
    const appInput = tryJsonParse(query.get('new'));
    const [createApi, setCreateApi] = React.useState(null);

    return (
<ClusterCtx.Provider value={services.clusters.list()}>
<Consumer>{
(ctx) => (
    <Page title='Applications' toolbar={services.viewPreferences.getPreferences().map((pref) => ({
        breadcrumbs: [{ title: 'Applications', path: '/applications' }],
        tools: (
            <React.Fragment key='app-list-tools'>
                <span className='applications-list__view-type'>
                    <i className={classNames('fa fa-th', {selected: pref.appList.view === 'tiles'})} onClick={() => {
                        ctx.navigation.goto('.', { view: 'tiles'});
                        services.viewPreferences.updatePreferences({ appList: {...pref.appList, view: 'tiles'} });
                    }} />
                    <i className={classNames('fa fa-th-list', {selected: pref.appList.view === 'list'})} onClick={() => {
                        ctx.navigation.goto('.', { view: 'list'});
                        services.viewPreferences.updatePreferences({ appList: {...pref.appList, view: 'list'} });
                    }} />
                    <i className={classNames('fa fa-chart-pie', {selected: pref.appList.view === 'summary'})} onClick={() => {
                        ctx.navigation.goto('.', { view: 'summary'});
                        services.viewPreferences.updatePreferences({ appList: {...pref.appList, view: 'summary'} });
                    }} />
                </span>
            </React.Fragment>
        ),
        actionMenu: {
            className: 'fa fa-plus',
            items: [{
                title: 'New Application',
                action: () => ctx.navigation.goto('.', { new: '{}' }),
            }],
        },
    }))}>
        <div className='applications-list'>
            <DataLoader
                load={() => loadApplications()}
                loadingRenderer={() => (<div className='argo-container'><MockupList height={100} marginTop={30}/></div>)}>
                {(applications: models.Application[]) => (
                    applications.length === 0 ? (
                        <EmptyState icon='argo-icon-application'>
                            <h4>No applications yet</h4>
                            <h5>Create new application to start managing resources in your cluster</h5>
                            <button className='argo-button argo-button--base' onClick={() => ctx.navigation.goto('.', { new: JSON.stringify({}) })}>Create application</button>
                        </EmptyState>
                    ) : (
                        <div className='row'>
                            <div className='columns small-12 xxlarge-2'>
                                <Query>
                                {(q) => (
                                    <div className='applications-list__search'>
                                        <i className='fa fa-search'/>
                                        {q.get('search') && (
                                            <i className='fa fa-times' onClick={() => ctx.navigation.goto('.', { search: null }, { replace: true })}/>
                                        )}
                                        <Autocomplete
                                            filterSuggestions={true}
                                            renderInput={(inputProps) => (
                                                <input {...inputProps} onFocus={(e) => {
                                                    e.target.select();
                                                    if (inputProps.onFocus) {
                                                        inputProps.onFocus(e);
                                                    }
                                                }} className='argo-field' />
                                            )}
                                            renderItem={(item) => (
                                                <React.Fragment>
                                                    <i className='icon argo-icon-application'/> {item.label}
                                                </React.Fragment>
                                            )}
                                            onSelect={(val) => {
                                                ctx.navigation.goto(`./${val}`);
                                            }}
                                            onChange={(e) => ctx.navigation.goto('.', { search: e.target.value }, { replace: true })}
                                            value={q.get('search') || ''} items={applications.map((app) => app.metadata.name)}/>
                                    </div>
                                )}
                                </Query>
                                <ViewPref>
                                {(pref) => <ApplicationsFilter applications={applications} pref={pref} onChange={(newPref) => {
                                    services.viewPreferences.updatePreferences({appList: newPref});
                                    ctx.navigation.goto('.', {
                                        proj: newPref.projectsFilter.join(','),
                                        sync: newPref.syncFilter.join(','),
                                        health: newPref.healthFilter.join(','),
                                        namespace: newPref.namespacesFilter.join(','),
                                        cluster: newPref.clustersFilter.join(','),
                                    });
                                }} />}
                                </ViewPref>
                            </div>
                            <div className='columns small-12 xxlarge-10'>
                            <ViewPref>
                            {(pref) => pref.view === 'summary' && (
                                <ApplicationsSummary applications={filterApps(applications, pref, pref.search)} />
                            ) || (
                                <Paginate
                                    preferencesKey='applications-list'
                                    page={pref.page}
                                    emptyState={() => (
                                        <EmptyState icon='fa fa-search'>
                                            <h4>No applications found</h4>
                                            <h5>Try to change filter criteria</h5>
                                        </EmptyState>
                                    )}
                                    data={filterApps(applications, pref, pref.search)} onPageChange={(page) => ctx.navigation.goto('.', { page })} >
                                {(data) => (
                                    pref.view === 'tiles' && (
                                        <ApplicationTiles
                                            applications={data}
                                            syncApplication={(appName) => ctx.navigation.goto('.', { syncApp: appName })}
                                            refreshApplication={(appName) => services.applications.get(appName, 'normal')}
                                            deleteApplication={(appName) => AppUtils.deleteApplication(appName, ctx)}
                                        />
                                    ) || (
                                        <ApplicationsTable applications={data}
                                            syncApplication={(appName) => ctx.navigation.goto('.', { syncApp: appName })}
                                            refreshApplication={(appName) => services.applications.get(appName, 'normal')}
                                            deleteApplication={(appName) => AppUtils.deleteApplication(appName, ctx)}
                                        />
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
        <ObservableQuery>
        {(q) => (
            <DataLoader load={() => q.flatMap((params) => {
                const syncApp = params.get('syncApp');
                return syncApp && Observable.fromPromise(services.applications.get(syncApp)) || Observable.from([null]);
             }) }>
            {(app) => (
                <ApplicationSyncPanel key='syncPanel' application={app} selectedResource={'all'} hide={() => ctx.navigation.goto('.', { syncApp: null })} />
            )}
            </DataLoader>
        )}
        </ObservableQuery>
        <SlidingPanel isMiddle={true} isShown={!!appInput} onClose={() => ctx.navigation.goto('.', { new: null })} header={
            <div>
                <button className='argo-button argo-button--base'
                        onClick={() => createApi && createApi.submitForm(null)}>
                    Create
                </button> <button onClick={() => ctx.navigation.goto('.', { new: null })} className='argo-button argo-button--base-o'>
                    Cancel
                </button>
            </div>
        }>
        {appInput && <ApplicationCreatePanel getFormApi={setCreateApi} createApp={ async (app) => {
                try {
                    await services.applications.create(app);
                    ctx.navigation.goto('.', { new: null });
                } catch (e) {
                    ctx.notifications.show({
                        content: <ErrorNotification title='Unable to create application' e={e}/>,
                        type: NotificationType.Error,
                    });
                }
            }}
            app={appInput}
            onAppChanged={(app) => ctx.navigation.goto('.', { new: JSON.stringify(app) }, { replace: true })} />}
        </SlidingPanel>
    </Page>
)}
</Consumer>
</ClusterCtx.Provider>);
};
