import {Autocomplete, ErrorNotification, MockupList, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Key, KeybindingContext, KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, merge, Observable} from 'rxjs';
import {bufferTime, delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {ClusterCtx, DataLoader, EmptyState, Page, Paginate, Spinner} from '../../../shared/components';
import {AuthSettingsCtx, Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppsListPreferences, AppsListViewKey, AppsListViewType, HealthStatusBarPreferences, services} from '../../../shared/services';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import * as AppUtils from '../utils';
import {ApplicationsFilter, FilteredApp, getAppFilterResults} from './applications-filter';
import {createMatcher} from './applications-list-search';
import {AppsStatusBar} from './applications-status-bar';
import {ApplicationsSummary} from './applications-summary';
import {ApplicationsTable} from './applications-table';
import {ApplicationTiles} from './applications-tiles';
import {ApplicationsRefreshPanel} from '../applications-refresh-panel/applications-refresh-panel';
import {FlexTopBar} from './flex-top-bar';
import {ViewTypeSwitcher} from './view-type-switcher';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {useQuery, useObservableQuery} from '../../../shared/hooks/query';

import './applications-list.scss';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

// The applications list/watch API supports only selected set of fields.
// Make sure to register any new fields in the `appFields` map of `pkg/apiclient/application/forwarder_overwrite.go`.
const APP_FIELDS = [
    'metadata.name',
    'metadata.namespace',
    'metadata.annotations',
    'metadata.labels',
    'metadata.creationTimestamp',
    'metadata.deletionTimestamp',
    'spec.destination',
    'spec.project',
    'spec.source',
    'spec.sources',
    'spec.sourceHydrator',
    'spec.syncPolicy',
    'operation.sync',
    'status.sourceHydrator',
    'status.summary',
    'status.sync.status',
    'status.sync.revision',
    'status.health',
    'status.operationState.phase',
    'status.operationState.startedAt',
    'status.operationState.finishedAt'
];
const APP_LIST_FIELDS = ['metadata.resourceVersion', ...APP_FIELDS.map(field => `items.${field}`)];
const APP_WATCH_FIELDS = ['result.type', ...APP_FIELDS.map(field => `result.application.${field}`)];

function loadApplications(projects: string[], appNamespace: string): Observable<models.Application[]> {
    return from(services.applications.list(projects, 'application', {appNamespace, fields: APP_LIST_FIELDS})).pipe(
        mergeMap(applicationsList => {
            const applications = applicationsList.items as models.Application[];
            return merge(
                from([applications]),
                services.applications
                    .watch('application', {projects, resourceVersion: applicationsList.metadata.resourceVersion}, {fields: APP_WATCH_FIELDS})
                    .pipe(repeat())
                    .pipe(retryWhen(errors => errors.pipe(delay(WATCH_RETRY_TIMEOUT))))
                    // batch events to avoid constant re-rendering and improve UI performance
                    .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
                    .pipe(
                        map(appChanges => {
                            appChanges.forEach(appChange => {
                                const index = applications.findIndex(item => AppUtils.appInstanceName(item) === AppUtils.appInstanceName(appChange.application));
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
                            });
                            return {applications, updated: appChanges.length > 0};
                        })
                    )
                    .pipe(filter(item => item.updated))
                    .pipe(map(item => item.applications))
            );
        })
    );
}

const ViewPref = ({children}: {children: (pref: AppsListPreferences & {page: number; search: string; searchRegex: boolean}) => React.ReactNode}) => {
    const observableQuery$ = useObservableQuery();

    return (
        <DataLoader
            load={() =>
                combineLatest([services.viewPreferences.getPreferences().pipe(map(item => item.appList)), observableQuery$]).pipe(
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
                        if (params.get('autoSync') != null) {
                            viewPref.autoSyncFilter = params
                                .get('autoSync')
                                .split(',')
                                .filter(item => !!item);
                        }
                        if (params.get('operation') != null) {
                            viewPref.operationFilter = params
                                .get('operation')
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
                        if (params.get('targetRevision') != null) {
                            viewPref.targetRevisionFilter = params
                                .get('targetRevision')
                                .split(',')
                                .map(decodeURIComponent)
                                .filter(item => !!item);
                        }
                        if (params.get('cluster') != null) {
                            viewPref.clustersFilter = params
                                .get('cluster')
                                .split(',')
                                .filter(item => !!item);
                        }
                        if (params.get('showFavorites') != null) {
                            viewPref.showFavorites = params.get('showFavorites') === 'true';
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
                        if (params.get('annotations') != null) {
                            viewPref.annotationsFilter = params
                                .get('annotations')
                                .split(',')
                                .map(decodeURIComponent)
                                .filter(item => !!item);
                        }
                        if (params.get('repo') != null) {
                            viewPref.reposFilter = params
                                .get('repo')
                                .split(',')
                                .map(decodeURIComponent)
                                .filter(item => !!item);
                        }
                        return {
                            ...viewPref,
                            page: parseInt(params.get('page') || '0', 10),
                            search: params.get('search') || '',
                            searchRegex: params.get('searchRegex') === 'true'
                        };
                    })
                )
            }>
            {pref => children(pref)}
        </DataLoader>
    );
};

function filterApplications(
    applications: models.Application[],
    pref: AppsListPreferences,
    search: string,
    searchRegex: boolean
): {filteredApps: models.Application[]; filterResults: FilteredApp[]} {
    const processedApps = applications.map(app => {
        let isAppOfAppsPattern = false;
        if (app.status.summary?.isAppOfApps === true) {
            isAppOfAppsPattern = true;
        }
        return {...app, isAppOfAppsPattern};
    });
    const filterResults = getAppFilterResults(processedApps, pref);
    const matchesSearch = createMatcher(search, searchRegex);

    return {
        filterResults,
        filteredApps: filterResults.filter(app => matchesSearch(app.metadata.name, app.metadata.namespace) && Object.values(app.filterResult).every(val => val))
    };
}

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

const SearchBar = (props: {content: string; searchRegex: boolean; ctx: ContextApis; apps: models.Application[]}) => {
    const {content, searchRegex, ctx, apps} = {...props};

    const regexValid = React.useMemo(() => {
        if (!searchRegex || !content) {
            return true;
        }
        try {
            new RegExp(content);
            return true;
        } catch {
            return false;
        }
    }, [searchRegex, content]);

    const searchBar = React.useRef<HTMLDivElement>(null);

    const query = new URLSearchParams(window.location.search);
    const appInput = tryJsonParse(query.get('new'));

    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (searchBar.current && !appInput) {
                searchBar.current.querySelector('input').focus();
                setFocus(true);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (searchBar.current && !appInput && isFocused) {
                searchBar.current.querySelector('input').blur();
                setFocus(false);
                return true;
            }
            return false;
        }
    });

    return (
        <Autocomplete
            filterSuggestions={true}
            renderInput={inputProps => (
                <div
                    className={classNames('applications-list__search', {
                        'applications-list__search--regex': searchRegex && regexValid,
                        'applications-list__search--regex-invalid': searchRegex && !regexValid
                    })}
                    ref={searchBar}>
                    <i
                        className='fa fa-search'
                        style={{marginRight: '9px', cursor: 'pointer'}}
                        onClick={() => {
                            if (searchBar.current) {
                                searchBar.current.querySelector('input').focus();
                            }
                        }}
                    />
                    <input
                        {...inputProps}
                        onFocus={e => {
                            e.target.select();
                            if (inputProps.onFocus) {
                                inputProps.onFocus(e);
                            }
                        }}
                        style={{fontSize: '14px'}}
                        className='argo-field'
                        placeholder={searchRegex ? 'Regex search (e.g. ^foo-.*-prod$)' : 'Search applications...'}
                    />
                    <Tooltip
                        content={searchRegex ? (regexValid ? 'Regex search enabled, click to switch to plain text' : 'Invalid regex pattern') : 'Click to enable regex search'}>
                        <button
                            type='button'
                            aria-label='Toggle regex search'
                            aria-pressed={searchRegex}
                            className={classNames('applications-list__regex-toggle', {
                                'applications-list__regex-toggle--active': searchRegex,
                                'applications-list__regex-toggle--invalid': searchRegex && !regexValid
                            })}
                            onClick={() => ctx.navigation.goto('.', {searchRegex: !searchRegex || null}, {replace: true})}>
                            .*
                        </button>
                    </Tooltip>
                    <div className='keyboard-hint'>/</div>
                    {content && (
                        <i className='fa fa-times' onClick={() => ctx.navigation.goto('.', {search: null}, {replace: true})} style={{cursor: 'pointer', marginLeft: '5px'}} />
                    )}
                </div>
            )}
            wrapperProps={{className: 'applications-list__search-wrapper'}}
            renderItem={item => (
                <React.Fragment>
                    <i className='icon argo-icon-application resource-icon__font-icon applications-list__search-resource-icon' />
                    <span>{item.label}</span>
                </React.Fragment>
            )}
            onSelect={val => {
                const selectedApp = apps?.find(app => {
                    const qualifiedName = AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);
                    return qualifiedName === val;
                });
                if (selectedApp) {
                    ctx.navigation.goto(`/${AppUtils.getAppUrl(selectedApp)}`);
                }
            }}
            onChange={e => ctx.navigation.goto('.', {search: e.target.value}, {replace: true})}
            value={content || ''}
            items={apps.map(app => AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled))}
        />
    );
};

interface ApplicationsToolbarProps {
    applications: models.Application[];
    pref: AppsListPreferences & {page: number; search: string; searchRegex: boolean};
    ctx: ContextApis;
    healthBarPrefs: HealthStatusBarPreferences;
}

const ApplicationsToolbar: React.FC<ApplicationsToolbarProps> = ({applications, pref, ctx, healthBarPrefs}) => {
    // Read searchRegex from the URL rather than from pref: the surrounding DataLoader caches its
    // children, so the toggle state has to come from a reactive source for the SearchBar to re-render.
    const query = useQuery();

    return (
        <React.Fragment key='app-list-tools'>
            <SearchBar content={query.get('search')} searchRegex={query.get('searchRegex') === 'true'} apps={applications} ctx={ctx} />
            <ViewTypeSwitcher pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />
        </React.Fragment>
    );
};

export const ApplicationsList = (props: RouteComponentProps<any>) => {
    const query = useQuery();
    const observableQuery$ = useObservableQuery();
    const appInput = tryJsonParse(query.get('new'));
    const syncAppsInput = tryJsonParse(query.get('syncApps'));
    const refreshAppsInput = tryJsonParse(query.get('refreshApps'));
    const [createApi, setCreateApi] = React.useState(null);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const [isAppCreatePending, setAppCreatePending] = React.useState(false);
    const loaderRef = React.useRef<DataLoader | null>(null);
    const {List, Summary, Tiles} = AppsListViewKey;

    function refreshApp(appName: string, appNamespace: string) {
        // app refreshing might be done too quickly so that UI might miss it due to event batching
        // add refreshing annotation in the UI to improve user experience
        if (loaderRef.current) {
            const applications = loaderRef.current.getData() as models.Application[];
            const app = applications.find(item => item.metadata.name === appName && item.metadata.namespace === appNamespace);
            if (app) {
                AppUtils.setAppRefreshing(app);
                loaderRef.current.setData(applications);
            }
        }
        services.applications.get(appName, appNamespace, 'application', 'normal');
    }

    function onAppFilterPrefChanged(ctx: ContextApis, newPref: AppsListPreferences) {
        services.viewPreferences.updatePreferences({appList: newPref});
        ctx.navigation.goto(
            '.',
            {
                proj: newPref.projectsFilter.join(','),
                sync: newPref.syncFilter.join(','),
                autoSync: newPref.autoSyncFilter.join(','),
                health: newPref.healthFilter.join(','),
                namespace: newPref.namespacesFilter.join(','),
                targetRevision: newPref.targetRevisionFilter.map(encodeURIComponent).join(','),
                repo: newPref.reposFilter.map(encodeURIComponent).join(','),
                cluster: newPref.clustersFilter.join(','),
                labels: newPref.labelsFilter.map(encodeURIComponent).join(','),
                annotations: newPref.annotationsFilter.map(encodeURIComponent).join(','),
                operation: newPref.operationFilter.join(','),
                // Keep URL and preferences consistent. When false, remove the param entirely.
                showFavorites: newPref.showFavorites ? 'true' : null
            },
            {replace: true}
        );
    }

    function getPageTitle(view: string) {
        switch (view) {
            case List:
                return 'Applications List';
            case Tiles:
                return 'Applications Tiles';
            case Summary:
                return 'Applications Summary';
        }
        return '';
    }

    const sidebarTarget = useSidebarTarget();

    return (
        <ClusterCtx.Provider value={clusters}>
            <KeybindingProvider>
                <Consumer>
                    {ctx => (
                        <ViewPref>
                            {pref => (
                                <Page
                                    key={pref.view}
                                    title={getPageTitle(pref.view)}
                                    useTitleOnly={true}
                                    toolbar={{
                                        breadcrumbs: [
                                            {
                                                title: 'Applications',
                                                path: props.match.url
                                            }
                                        ]
                                    }}
                                    hideAuth={true}>
                                    <DataLoader
                                        input={pref.projectsFilter?.join(',')}
                                        ref={loaderRef}
                                        load={() => AppUtils.handlePageVisibility(() => loadApplications(pref.projectsFilter, query.get('appNamespace')))}
                                        loadingRenderer={() => (
                                            <div className='argo-container'>
                                                <MockupList height={100} marginTop={30} />
                                            </div>
                                        )}>
                                        {(applications: models.Application[]) => {
                                            const healthBarPrefs = pref.statusBarView || ({} as HealthStatusBarPreferences);
                                            const handleCreatePanelClose = async () => {
                                                const outsideDiv = document.querySelector('.sliding-panel__outside');
                                                const closeButton = document.querySelector('.sliding-panel__close');

                                                if (outsideDiv && closeButton && closeButton !== document.activeElement) {
                                                    const confirmed = await ctx.popup.confirm('Close Panel', 'Closing this panel will discard all entered values. Continue?');
                                                    if (confirmed) {
                                                        ctx.navigation.goto('.', {new: null}, {replace: true});
                                                    }
                                                } else if (closeButton === document.activeElement) {
                                                    // If the close button is focused or clicked, close without confirmation
                                                    ctx.navigation.goto('.', {new: null}, {replace: true});
                                                }
                                            };

                                            const apps = applications as models.Application[];
                                            const {filteredApps, filterResults} = filterApplications(apps, pref, pref.search, pref.searchRegex);

                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        toolbar={{
                                                            tools: <ApplicationsToolbar applications={applications} pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />,
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
                                                        {apps.length === 0 && pref.projectsFilter?.length === 0 && (pref.labelsFilter || []).length === 0 ? (
                                                            <EmptyState icon='argo-icon-application'>
                                                                <h4>No applications available to you just yet</h4>
                                                                <h5>Create new application to start managing resources in your cluster</h5>
                                                                <button
                                                                    qe-id='applications-list-button-create-application'
                                                                    className='argo-button argo-button--base'
                                                                    onClick={() => ctx.navigation.goto('.', {new: JSON.stringify({})}, {replace: true})}>
                                                                    Create application
                                                                </button>
                                                            </EmptyState>
                                                        ) : (
                                                            <>
                                                                {ReactDOM.createPortal(
                                                                    <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                        {allpref => (
                                                                            <ApplicationsFilter
                                                                                apps={filterResults}
                                                                                onChange={newPrefs => onAppFilterPrefChanged(ctx, newPrefs)}
                                                                                pref={pref}
                                                                                collapsed={allpref.hideSidebar}
                                                                            />
                                                                        )}
                                                                    </DataLoader>,
                                                                    sidebarTarget?.current
                                                                )}

                                                                {(pref.view === 'summary' && <ApplicationsSummary applications={filteredApps} />) || (
                                                                    <Paginate
                                                                        header={filteredApps.length > 1 && <AppsStatusBar applications={filteredApps} />}
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
                                                                                            onAppFilterPrefChanged(ctx, pref);
                                                                                        }}>
                                                                                        clear filters
                                                                                    </a>
                                                                                </h5>
                                                                            </EmptyState>
                                                                        )}
                                                                        sortOptions={[
                                                                            {
                                                                                title: 'Name',
                                                                                compare: (a, b) => a.metadata.name.localeCompare(b.metadata.name, undefined, {numeric: true})
                                                                            },
                                                                            {
                                                                                title: 'Created At',
                                                                                compare: (b, a) => a.metadata.creationTimestamp.localeCompare(b.metadata.creationTimestamp)
                                                                            },
                                                                            {
                                                                                title: 'Synchronized',
                                                                                compare: (b, a) =>
                                                                                    a.status.operationState?.finishedAt?.localeCompare(b.status.operationState?.finishedAt)
                                                                            }
                                                                        ]}
                                                                        data={filteredApps}
                                                                        onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                        {data =>
                                                                            (pref.view === 'tiles' && (
                                                                                <ApplicationTiles
                                                                                    applications={data}
                                                                                    syncApplication={(appName, appNamespace) =>
                                                                                        ctx.navigation.goto('.', {syncApp: appName, appNamespace}, {replace: true})
                                                                                    }
                                                                                    refreshApplication={refreshApp}
                                                                                    deleteApplication={(appName, appNamespace) =>
                                                                                        AppUtils.deleteApplication(appName, appNamespace, ctx)
                                                                                    }
                                                                                />
                                                                            )) || (
                                                                                <ApplicationsTable
                                                                                    applications={data}
                                                                                    syncApplication={(appName, appNamespace) =>
                                                                                        ctx.navigation.goto('.', {syncApp: appName, appNamespace}, {replace: true})
                                                                                    }
                                                                                    refreshApplication={refreshApp}
                                                                                    deleteApplication={(appName, appNamespace) =>
                                                                                        AppUtils.deleteApplication(appName, appNamespace, ctx)
                                                                                    }
                                                                                />
                                                                            )
                                                                        }
                                                                    </Paginate>
                                                                )}
                                                            </>
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
                                                    <DataLoader
                                                        load={() =>
                                                            observableQuery$.pipe(
                                                                mergeMap(params => {
                                                                    const syncApp = params.get('syncApp');
                                                                    const appNamespace = params.get('appNamespace');
                                                                    return (syncApp && from(services.applications.get(syncApp, appNamespace, 'application'))) || from([null]);
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
                                                    <SlidingPanel
                                                        isShown={!!appInput}
                                                        onClose={() => handleCreatePanelClose()}
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
                                </Page>
                            )}
                        </ViewPref>
                    )}
                </Consumer>
            </KeybindingProvider>
        </ClusterCtx.Provider>
    );
};
