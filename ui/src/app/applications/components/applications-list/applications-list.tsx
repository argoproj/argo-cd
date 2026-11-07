import {ErrorNotification, MockupList, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from} from 'rxjs';
import {map, mergeMap} from 'rxjs/operators';
import {ClusterCtx, DataLoader, EmptyState, Page, SearchBar, Spinner} from '../../../shared/components';
import {lazyWithBoundary} from '../../../shared/components/lazy-with-boundary';
import {AuthSettingsCtx, Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppsListPreferences, AppsListViewKey, AppsListViewType, HealthStatusBarPreferences, services} from '../../../shared/services';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import * as AppUtils from '../utils';
import {ApplicationsFilter} from './applications-filter';
import {AppsListPaginate, useApplicationsList} from './applications-list-client';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';
import {AppsStatusBar} from './applications-status-bar';
import {ApplicationsTable} from './applications-table';
import {ApplicationTiles} from './applications-tiles';
import {ApplicationsRefreshPanel} from '../applications-refresh-panel/applications-refresh-panel';
import {FlexTopBar} from '../../../shared/components';
import {ViewTypeSwitcher} from './view-type-switcher';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {useSearchParams, useObservableQuery} from '../../../shared/hooks/query';
import {isInvalidRegex} from '../../../shared/utils';
import {loadFailed} from '../../../shared/query-client';

import './applications-list.scss';

const ApplicationCreatePanel = lazyWithBoundary(
    React.lazy(() => import(/* webpackChunkName: "app-create-panel" */ '../application-create-panel/application-create-panel').then(m => ({default: m.ApplicationCreatePanel}))),
    'Failed to load application create panel. Please reload and try again.'
);
const ApplicationsSummary = lazyWithBoundary(
    React.lazy(() => import(/* webpackChunkName: "apps-summary" */ './applications-summary').then(m => ({default: m.ApplicationsSummary}))),
    'Failed to load applications summary. Please reload and try again.'
);

const ViewPref = ({children}: {children: (pref: AppsListPreferences & {page: number; search: string}) => React.ReactNode}) => {
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
                        if (params.get('health') != null) {
                            viewPref.healthFilter = params
                                .get('health')
                                .split(',')
                                .filter(item => !!item);
                        }
                        if (params.get('hydration') != null) {
                            viewPref.hydrationFilter = params
                                .get('hydration')
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
                            search: params.get('search') || ''
                        };
                    })
                )
            }>
            {pref => children(pref)}
        </DataLoader>
    );
};

// Render-prop wrapper so the list body, which still sits inside render callbacks, can use the hook.
const AppsListData = ({children, ...query}: AppsListSourceQuery & {children: (list: AppsListSourceResult) => React.ReactNode}) => (
    <React.Fragment>{children(useApplicationsList(query))}</React.Fragment>
);

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

const ApplicationsListSearchBar = (props: {content: string; searchRegex: boolean; ctx: ContextApis; apps: models.Application[]}) => {
    const {content, searchRegex, ctx, apps} = props;
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    const query = new URLSearchParams(window.location.search);
    const appInput = tryJsonParse(query.get('new'));

    return (
        <SearchBar
            value={content || ''}
            onChange={value => ctx.navigation.goto('.', {search: value}, {replace: true})}
            placeholder={searchRegex ? 'Regex search (e.g. ^foo-.*-prod$)' : 'Search applications...'}
            disableKeyboardShortcuts={!!appInput}
            regexEnabled={searchRegex}
            autocomplete={{
                items: apps.map(app => AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)),
                filterSuggestions: true,
                onSelect: val => {
                    const selectedApp = apps?.find(app => {
                        const qualifiedName = AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);
                        return qualifiedName === val;
                    });
                    if (selectedApp) {
                        ctx.navigation.goto(`/${AppUtils.getAppUrl(selectedApp)}`);
                    }
                },
                renderItem: item => (
                    <React.Fragment>
                        <i className='icon argo-icon-application' style={{flexShrink: 0}} />
                        <span title={item.label} style={{overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                            {item.label}
                        </span>
                    </React.Fragment>
                )
            }}
        />
    );
};

interface ApplicationsToolbarProps {
    applications: models.Application[];
    pref: AppsListPreferences & {page: number; search: string};
    ctx: ContextApis;
    healthBarPrefs: HealthStatusBarPreferences;
}

const ApplicationsToolbar: React.FC<ApplicationsToolbarProps> = ({applications, pref, ctx, healthBarPrefs}) => {
    const regexInvalid = pref.searchRegex && isInvalidRegex(pref.search);

    const regexToggleClass = `applications-list__regex-toggle argo-button argo-button--base${pref.searchRegex ? '' : '-o'}${
        regexInvalid ? ' applications-list__regex-toggle--invalid' : ''
    }`;

    return (
        <div className='applications-list__toolbar-controls' key='app-list-tools'>
            <ApplicationsListSearchBar content={pref.search} searchRegex={pref.searchRegex} apps={applications} ctx={ctx} />
            <Tooltip content={pref.searchRegex ? (regexInvalid ? 'Invalid regex pattern' : 'Regex search enabled, click to switch to plain text') : 'Click to enable regex search'}>
                <button
                    type='button'
                    aria-label='Toggle regex search'
                    aria-pressed={pref.searchRegex}
                    className={regexToggleClass}
                    onClick={() => {
                        services.viewPreferences.updatePreferences({
                            appList: {...pref, searchRegex: !pref.searchRegex}
                        });
                    }}>
                    .*
                </button>
            </Tooltip>
            <Tooltip content='Toggle Health Status Bar'>
                <button
                    className={`applications-list__accordion argo-button argo-button--base${healthBarPrefs.showHealthStatusBar ? '-o' : ''}`}
                    style={{border: 'none'}}
                    onClick={() => {
                        const showHealthStatusBar = !healthBarPrefs.showHealthStatusBar;
                        services.viewPreferences.updatePreferences({
                            appList: {
                                ...pref,
                                statusBarView: {
                                    ...healthBarPrefs,
                                    showHealthStatusBar
                                }
                            }
                        });
                    }}>
                    <i className='fas fa-ruler-horizontal' />
                </button>
            </Tooltip>
        </div>
    );
};

export const ApplicationsList = (props: RouteComponentProps<any>) => {
    const query = useSearchParams();
    const observableQuery$ = useObservableQuery();
    const appInput = tryJsonParse(query.get('new'));
    const syncAppsInput = tryJsonParse(query.get('syncApps'));
    const refreshAppsInput = tryJsonParse(query.get('refreshApps'));
    const [createApi, setCreateApi] = React.useState(null);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const [isAppCreatePending, setAppCreatePending] = React.useState(false);
    const {List, Summary, Tiles} = AppsListViewKey;
    const authSettings = React.useContext(AuthSettingsCtx);

    function onAppFilterPrefChanged(ctx: ContextApis, newPref: AppsListPreferences) {
        services.viewPreferences.updatePreferences({appList: newPref});
        const params = {
            proj: newPref.projectsFilter.join(','),
            sync: newPref.syncFilter.join(','),
            autoSync: newPref.autoSyncFilter.join(','),
            health: newPref.healthFilter.join(','),
            hydration: newPref.hydrationFilter.join(','),
            namespace: newPref.namespacesFilter.join(','),
            targetRevision: newPref.targetRevisionFilter.map(encodeURIComponent).join(','),
            repo: newPref.reposFilter.map(encodeURIComponent).join(','),
            cluster: newPref.clustersFilter.join(','),
            labels: newPref.labelsFilter.map(encodeURIComponent).join(','),
            annotations: newPref.annotationsFilter.map(encodeURIComponent).join(','),
            operation: newPref.operationFilter.join(','),
            // Keep URL and preferences consistent. When false, remove the param entirely.
            showFavorites: newPref.showFavorites ? 'true' : null
        };
        // Every filter in the sidebar reports its state up when it mounts, so a remount produces one
        // call per filter with nothing changed. Browsers rate limit history updates and Safari throws
        // once the cap is hit, so only navigate when the query string actually changes.
        ctx.navigation.goto('.', params, {replace: true, skipIfUnchanged: true});
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
                                    }}>
                                    <AppsListData pref={pref} appNamespace={query.get('appNamespace')} hydratorEnabled={authSettings?.hydratorEnabled}>
                                        {list => {
                                            if (list.loading) {
                                                return (
                                                    <div className='argo-container'>
                                                        <MockupList height={100} marginTop={30} />
                                                    </div>
                                                );
                                            }
                                            if (list.failed) {
                                                return loadFailed(list.retry);
                                            }
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

                                            const {rows: filteredApps, facets, stats} = list;

                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        key={`toolbar-${healthBarPrefs.showHealthStatusBar}-${pref.view}`}
                                                        toolbar={{
                                                            tools: (
                                                                <ApplicationsToolbar applications={list.searchSuggestions} pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />
                                                            ),
                                                            options: <ViewTypeSwitcher pref={pref} ctx={ctx} />,
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
                                                        {list.noApplications ? (
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
                                                                                facets={facets}
                                                                                onChange={newPrefs => onAppFilterPrefChanged(ctx, newPrefs)}
                                                                                pref={pref}
                                                                                collapsed={allpref.hideSidebar}
                                                                                hydratorEnabled={authSettings?.hydratorEnabled}
                                                                            />
                                                                        )}
                                                                    </DataLoader>,
                                                                    sidebarTarget?.current
                                                                )}

                                                                {(pref.view === 'summary' && <ApplicationsSummary stats={stats} />) || (
                                                                    <AppsListPaginate
                                                                        list={list}
                                                                        header={stats.total > 1 && <AppsStatusBar stats={stats} />}
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
                                                                        onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                        {(data, useVirtualScrolling) =>
                                                                            (pref.view === 'tiles' && (
                                                                                <ApplicationTiles
                                                                                    applications={data}
                                                                                    syncApplication={(appName, appNamespace) =>
                                                                                        ctx.navigation.goto(
                                                                                            '.',
                                                                                            {syncApp: appName, syncAppNamespace: appNamespace},
                                                                                            {replace: true}
                                                                                        )
                                                                                    }
                                                                                    refreshApplication={list.markRefreshing}
                                                                                    deleteApplication={(appName, appNamespace) =>
                                                                                        AppUtils.deleteApplication(appName, appNamespace, ctx)
                                                                                    }
                                                                                    useVirtualScrolling={useVirtualScrolling}
                                                                                    statusBarVisible={healthBarPrefs.showHealthStatusBar}
                                                                                />
                                                                            )) || (
                                                                                <ApplicationsTable
                                                                                    applications={data}
                                                                                    syncApplication={(appName, appNamespace) =>
                                                                                        ctx.navigation.goto(
                                                                                            '.',
                                                                                            {syncApp: appName, syncAppNamespace: appNamespace},
                                                                                            {replace: true}
                                                                                        )
                                                                                    }
                                                                                    refreshApplication={list.markRefreshing}
                                                                                    deleteApplication={(appName, appNamespace) =>
                                                                                        AppUtils.deleteApplication(appName, appNamespace, ctx)
                                                                                    }
                                                                                    useVirtualScrolling={useVirtualScrolling}
                                                                                    statusBarVisible={healthBarPrefs.showHealthStatusBar}
                                                                                />
                                                                            )
                                                                        }
                                                                    </AppsListPaginate>
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
                                                                    const appNamespace = params.get('syncAppNamespace');
                                                                    return (syncApp && from(services.applications.get(syncApp, appNamespace, 'application'))) || from([null]);
                                                                })
                                                            )
                                                        }>
                                                        {app => (
                                                            <ApplicationSyncPanel
                                                                key='syncPanel'
                                                                application={app}
                                                                selectedResource={'all'}
                                                                hide={() => ctx.navigation.goto('.', {syncApp: null, syncAppNamespace: null}, {replace: true})}
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
                                    </AppsListData>
                                </Page>
                            )}
                        </ViewPref>
                    )}
                </Consumer>
            </KeybindingProvider>
        </ClusterCtx.Provider>
    );
};
