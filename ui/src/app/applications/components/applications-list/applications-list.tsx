import {ErrorNotification, MockupList, NotificationType, SlidingPanel, Toolbar, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, Observable} from 'rxjs';
import {map, mergeMap} from 'rxjs/operators';
import {AddAuthToToolbar, ClusterCtx, DataLoader, EmptyState, ObservableQuery, Page, Paginate, Query, Spinner} from '../../../shared/components';
import {Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppsListPreferences, AppsListViewType, HealthStatusBarPreferences, services} from '../../../shared/services';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import * as AppUtils from '../utils';
import {ApplicationsFilter, FilteredApp, getFilterResults} from './applications-filter';
import {ApplicationsStatusBar} from './applications-status-bar';
import {ApplicationsSummary} from './applications-summary';
import {ApplicationsTable} from './applications-table';
import {ApplicationTiles} from './applications-tiles';
import {SearchBar} from '../../../shared/components/search-bar';
import {ApplicationsRefreshPanel} from '../applications-refresh-panel/applications-refresh-panel';

require('./applications-list.scss');
require('./flex-top-bar.scss');

export const ViewPref = ({children}: {children: (pref: AppsListPreferences & {page: number; search: string}) => React.ReactNode}) => (
    <ObservableQuery>
        {q => (
            <DataLoader
                load={() =>
                    combineLatest([services.viewPreferences.getPreferences().pipe(map(item => item.appList)), q]).pipe(
                        map(items => {
                            const params = items[1];
                            const viewPref: AppsListPreferences = {...items[0]};
                            if (params.get('proj') != null) {
                                viewPref.projectsFilter = params
                                    .get('proj')
                                    .split(',')
                                    .filter(item => !!item);
                            }
                            if (params.get('sync') != null) {
                                viewPref.syncFilter = params
                                    .get('sync')
                                    .split(',')
                                    .filter(item => !!item);
                            }
                            if (params.get('health') != null) {
                                viewPref.healthFilter = params
                                    .get('health')
                                    .split(',')
                                    .filter(item => !!item);
                            }
                            if (params.get('namespace') != null) {
                                viewPref.namespacesFilter = params
                                    .get('namespace')
                                    .split(',')
                                    .filter(item => !!item);
                            }
                            if (params.get('cluster') != null) {
                                viewPref.clustersFilter = params
                                    .get('cluster')
                                    .split(',')
                                    .filter(item => !!item);
                            }
                            if (params.get('view') != null) {
                                viewPref.view = params.get('view') as AppsListViewType;
                            }
                            if (params.get('labels') != null) {
                                viewPref.labelsFilter = params
                                    .get('labels')
                                    .split(',')
                                    .map(decodeURIComponent)
                                    .filter(item => !!item);
                            }
                            return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || ''};
                        })
                    )
                }>
                {pref => children(pref)}
            </DataLoader>
        )}
    </ObservableQuery>
);

function filterApps(applications: models.Application[], pref: AppsListPreferences, search: string): {filteredApps: models.Application[]; filterResults: FilteredApp[]} {
    const filterResults = getFilterResults(applications, pref);
    return {
        filterResults,
        filteredApps: filterResults.filter(app => (search === '' || app.metadata.name.includes(search)) && Object.values(app.filterResult).every(val => val))
    };
}

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

const FlexTopBar = (props: {toolbar: Toolbar | Observable<Toolbar>}) => {
    const ctx = React.useContext(Context);
    const loadToolbar = AddAuthToToolbar(props.toolbar, ctx);
    return (
        <React.Fragment>
            <div className='top-bar row flex-top-bar' key='tool-bar'>
                <DataLoader load={() => loadToolbar}>
                    {toolbar => (
                        <React.Fragment>
                            <div className='flex-top-bar__actions'>
                                {toolbar.actionMenu && (
                                    <React.Fragment>
                                        {toolbar.actionMenu.items.map((item, i) => (
                                            <button
                                                disabled={!!item.disabled}
                                                qe-id={item.qeId}
                                                className='argo-button argo-button--base'
                                                onClick={() => item.action()}
                                                style={{marginRight: 2}}
                                                key={i}>
                                                {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                                                {item.title}
                                            </button>
                                        ))}
                                    </React.Fragment>
                                )}
                            </div>
                            <div className='flex-top-bar__tools'>{toolbar.tools}</div>
                        </React.Fragment>
                    )}
                </DataLoader>
            </div>
            <div className='flex-top-bar__padder' />
        </React.Fragment>
    );
};

export const ApplicationsList = (props: RouteComponentProps<{}>) => {
    const query = new URLSearchParams(props.location.search);
    const appInput = tryJsonParse(query.get('new'));
    const syncAppsInput = tryJsonParse(query.get('syncApps'));
    const refreshAppsInput = tryJsonParse(query.get('refreshApps'));
    const [createApi, setCreateApi] = React.useState(null);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const [isAppCreatePending, setAppCreatePending] = React.useState(false);
    const loaderRef = React.useRef<DataLoader>();

    function refreshApp(appName: string) {
        // app refreshing might be done too quickly so that UI might miss it due to event batching
        // add refreshing annotation in the UI to improve user experience
        if (loaderRef.current) {
            const applications = loaderRef.current.getData() as models.Application[];
            const app = applications.find(item => item.metadata.name === appName);
            if (app) {
                AppUtils.setAppRefreshing(app);
                loaderRef.current.setData(applications);
            }
        }
        services.applications.get(appName, 'normal');
    }

    function onFilterPrefChanged(ctx: ContextApis, newPref: AppsListPreferences) {
        services.viewPreferences.updatePreferences({appList: newPref});
        ctx.navigation.goto(
            '.',
            {
                proj: newPref.projectsFilter.join(','),
                sync: newPref.syncFilter.join(','),
                health: newPref.healthFilter.join(','),
                namespace: newPref.namespacesFilter.join(','),
                cluster: newPref.clustersFilter.join(','),
                labels: newPref.labelsFilter.map(encodeURIComponent).join(',')
            },
            {replace: true}
        );
    }

    return (
        <ClusterCtx.Provider value={clusters}>
            <KeybindingProvider>
                <Consumer>
                    {ctx => (
                        <Page title='Applications' toolbar={{breadcrumbs: [{title: 'Applications', path: '/applications'}]}} hideAuth={true}>
                            <ViewPref>
                                {pref => (
                                    <DataLoader
                                        input={pref.projectsFilter?.join(',')}
                                        ref={loaderRef}
                                        load={() => AppUtils.handlePageVisibility(() => AppUtils.loadApplications(pref.projectsFilter))}
                                        loadingRenderer={() => (
                                            <div className='argo-container'>
                                                <MockupList height={100} marginTop={30} />
                                            </div>
                                        )}>
                                        {(applications: models.Application[]) => {
                                            const healthBarPrefs = pref.statusBarView || ({} as HealthStatusBarPreferences);
                                            const {filteredApps, filterResults} = filterApps(applications, pref, pref.search);
                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        toolbar={{
                                                            tools: (
                                                                <React.Fragment key='app-list-tools'>
                                                                    <Query>{q => <SearchBar content={q.get('search')} apps={applications} ctx={ctx} />}</Query>
                                                                    <Tooltip content='Toggle Health Status Bar'>
                                                                        <button
                                                                            className={`applications-list__accordion argo-button argo-button--base${
                                                                                healthBarPrefs.showHealthStatusBar ? '-o' : ''
                                                                            }`}
                                                                            style={{border: 'none'}}
                                                                            onClick={() =>
                                                                                services.viewPreferences.updatePreferences({
                                                                                    appList: {
                                                                                        ...pref,
                                                                                        statusBarView: {
                                                                                            ...healthBarPrefs,
                                                                                            showHealthStatusBar: !healthBarPrefs.showHealthStatusBar
                                                                                        }
                                                                                    }
                                                                                })
                                                                            }>
                                                                            <i className={`fas fa-ruler-horizontal`} />
                                                                        </button>
                                                                    </Tooltip>
                                                                    <div className='applications-list__view-type' style={{marginLeft: 'auto'}}>
                                                                        <i
                                                                            className={classNames('fa fa-th', {selected: pref.view === 'tiles'}, 'menu_icon')}
                                                                            title='Tiles'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: 'tiles'}, {replace: true});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: 'tiles'}});
                                                                            }}
                                                                        />
                                                                        <i
                                                                            className={classNames('fa fa-th-list', {selected: pref.view === 'list'}, 'menu_icon')}
                                                                            title='List'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: 'list'}, {replace: true});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: 'list'}});
                                                                            }}
                                                                        />
                                                                        <i
                                                                            className={classNames('fa fa-chart-pie', {selected: pref.view === 'summary'}, 'menu_icon')}
                                                                            title='Summary'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: 'summary'}, {replace: true});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: 'summary'}});
                                                                            }}
                                                                        />
                                                                    </div>
                                                                </React.Fragment>
                                                            ),
                                                            actionMenu: {
                                                                items: [
                                                                    {
                                                                        title: 'New App',
                                                                        iconClassName: 'fa fa-plus',
                                                                        qeId: 'applications-list-button-new-app',
                                                                        action: () => ctx.navigation.goto('.', {new: '{}'}, {replace: true})
                                                                    },
                                                                    {
                                                                        title: 'Sync Apps',
                                                                        iconClassName: 'fa fa-sync',
                                                                        action: () => ctx.navigation.goto('.', {syncApps: true}, {replace: true})
                                                                    },
                                                                    {
                                                                        title: 'Refresh Apps',
                                                                        iconClassName: 'fa fa-redo',
                                                                        action: () => ctx.navigation.goto('.', {refreshApps: true}, {replace: true})
                                                                    }
                                                                ]
                                                            }
                                                        }}
                                                    />
                                                    <div className='applications-list'>
                                                        {applications.length === 0 && pref.projectsFilter?.length === 0 && (pref.labelsFilter || []).length === 0 ? (
                                                            <EmptyState icon='argo-icon-application'>
                                                                <h4>No applications yet</h4>
                                                                <h5>Create new application to start managing resources in your cluster</h5>
                                                                <button
                                                                    qe-id='applications-list-button-create-application'
                                                                    className='argo-button argo-button--base'
                                                                    onClick={() => ctx.navigation.goto('.', {new: JSON.stringify({})}, {replace: true})}>
                                                                    Create application
                                                                </button>
                                                            </EmptyState>
                                                        ) : (
                                                            <ApplicationsFilter apps={filterResults} onChange={newPrefs => onFilterPrefChanged(ctx, newPrefs)} pref={pref}>
                                                                {(pref.view === 'summary' && <ApplicationsSummary applications={filteredApps} />) || (
                                                                    <Paginate
                                                                        header={filteredApps.length > 1 && <ApplicationsStatusBar applications={filteredApps} />}
                                                                        showHeader={healthBarPrefs.showHealthStatusBar}
                                                                        preferencesKey='applications-list'
                                                                        page={pref.page}
                                                                        emptyState={() => (
                                                                            <EmptyState icon='fa fa-search'>
                                                                                <h4>No matching applications found</h4>
                                                                                <h5>
                                                                                    Change filter criteria or&nbsp;
                                                                                    <a
                                                                                        onClick={() => {
                                                                                            AppsListPreferences.clearFilters(pref);
                                                                                            onFilterPrefChanged(ctx, pref);
                                                                                        }}>
                                                                                        clear filters
                                                                                    </a>
                                                                                </h5>
                                                                            </EmptyState>
                                                                        )}
                                                                        data={filteredApps}
                                                                        onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                        {data =>
                                                                            (pref.view === 'tiles' && (
                                                                                <ApplicationTiles
                                                                                    applications={data}
                                                                                    syncApplication={appName => ctx.navigation.goto('.', {syncApp: appName}, {replace: true})}
                                                                                    refreshApplication={refreshApp}
                                                                                    deleteApplication={appName => AppUtils.deleteApplication(appName, ctx)}
                                                                                />
                                                                            )) || (
                                                                                <ApplicationsTable
                                                                                    applications={data}
                                                                                    syncApplication={appName => ctx.navigation.goto('.', {syncApp: appName}, {replace: true})}
                                                                                    refreshApplication={refreshApp}
                                                                                    deleteApplication={appName => AppUtils.deleteApplication(appName, ctx)}
                                                                                />
                                                                            )
                                                                        }
                                                                    </Paginate>
                                                                )}
                                                            </ApplicationsFilter>
                                                        )}
                                                        <ApplicationsSyncPanel
                                                            key='syncsPanel'
                                                            show={syncAppsInput}
                                                            hide={() => ctx.navigation.goto('.', {syncApps: null}, {replace: true})}
                                                            apps={filteredApps}
                                                        />
                                                        <ApplicationsRefreshPanel
                                                            key='refreshPanel'
                                                            show={refreshAppsInput}
                                                            hide={() => ctx.navigation.goto('.', {refreshApps: null}, {replace: true})}
                                                            apps={filteredApps}
                                                        />
                                                    </div>
                                                    <ObservableQuery>
                                                        {q => (
                                                            <DataLoader
                                                                load={() =>
                                                                    q.pipe(
                                                                        mergeMap(params => {
                                                                            const syncApp = params.get('syncApp');
                                                                            return (syncApp && from(services.applications.get(syncApp))) || from([null]);
                                                                        })
                                                                    )
                                                                }>
                                                                {app => (
                                                                    <ApplicationSyncPanel
                                                                        key='syncPanel'
                                                                        application={app}
                                                                        selectedResource={'all'}
                                                                        hide={() => ctx.navigation.goto('.', {syncApp: null}, {replace: true})}
                                                                    />
                                                                )}
                                                            </DataLoader>
                                                        )}
                                                    </ObservableQuery>
                                                    <SlidingPanel
                                                        isShown={!!appInput}
                                                        onClose={() => ctx.navigation.goto('.', {new: null}, {replace: true})}
                                                        header={
                                                            <div>
                                                                <button
                                                                    qe-id='applications-list-button-create'
                                                                    className='argo-button argo-button--base'
                                                                    disabled={isAppCreatePending}
                                                                    onClick={() => createApi && createApi.submitForm(null)}>
                                                                    <Spinner show={isAppCreatePending} style={{marginRight: '5px'}} />
                                                                    Create
                                                                </button>{' '}
                                                                <button
                                                                    qe-id='applications-list-button-cancel'
                                                                    onClick={() => ctx.navigation.goto('.', {new: null}, {replace: true})}
                                                                    className='argo-button argo-button--base-o'>
                                                                    Cancel
                                                                </button>
                                                            </div>
                                                        }>
                                                        {appInput && (
                                                            <ApplicationCreatePanel
                                                                getFormApi={api => {
                                                                    setCreateApi(api);
                                                                }}
                                                                createApp={async app => {
                                                                    setAppCreatePending(true);
                                                                    try {
                                                                        await services.applications.create(app);
                                                                        ctx.navigation.goto('.', {new: null}, {replace: true});
                                                                    } catch (e) {
                                                                        ctx.notifications.show({
                                                                            content: <ErrorNotification title='Unable to create application' e={e} />,
                                                                            type: NotificationType.Error
                                                                        });
                                                                    } finally {
                                                                        setAppCreatePending(false);
                                                                    }
                                                                }}
                                                                app={appInput}
                                                                onAppChanged={app => ctx.navigation.goto('.', {new: JSON.stringify(app)}, {replace: true})}
                                                            />
                                                        )}
                                                    </SlidingPanel>
                                                </React.Fragment>
                                            );
                                        }}
                                    </DataLoader>
                                )}
                            </ViewPref>
                        </Page>
                    )}
                </Consumer>
            </KeybindingProvider>
        </ClusterCtx.Provider>
    );
};
