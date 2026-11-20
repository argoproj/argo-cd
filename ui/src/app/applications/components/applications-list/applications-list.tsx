import {ErrorNotification, MockupList, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import {KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {from} from 'rxjs';
import {mergeMap} from 'rxjs/operators';
import {ClusterCtx, DataLoader, EmptyState, Page, Spinner, useRefreshClusterList} from '../../../shared/components';
import {lazyWithBoundary} from '../../../shared/components/lazy-with-boundary';
import {AuthSettingsCtx, Consumer} from '../../../shared/context';
import {AppsListPreferences, AppsListViewKey, AppsListViewType, HealthStatusBarPreferences, services} from '../../../shared/services';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import * as AppUtils from '../utils';
import {ApplicationsQueryBar} from './applications-query-bar';
import {applicationQueryParams} from './applications-query-url';
import {AppsListPaginate, useApplicationsList} from './applications-list-client';
import {AppsListSourceQuery, AppsListSourceResult} from './applications-list-source';
import {AppsStatusBar} from './applications-status-bar';
import {ApplicationsTable} from './applications-table';
import {ApplicationTiles} from './applications-tiles';
import {ApplicationsRefreshPanel} from '../applications-refresh-panel/applications-refresh-panel';
import {FlexTopBar} from '../../../shared/components';
import {ViewTypeSwitcher} from './view-type-switcher';
import {useLocationSearch, useSearchParams, useObservableQuery} from '../../../shared/hooks/query';
import {useViewPreferences} from '../../../shared/hooks/use-view-preferences';
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
    const appList = useViewPreferences(pref => pref.appList);
    const search = useLocationSearch();
    const pref = React.useMemo(() => {
        const params = new URLSearchParams(search);
        return {
            ...appList,
            view: (params.get('view') || appList.view) as AppsListViewType,
            page: parseInt(params.get('page') || '0', 10),
            search: params.get('q') ?? params.get('search') ?? ''
        };
    }, [appList, search]);
    return <>{children(pref)}</>;
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

interface ApplicationsToolbarProps {
    pref: AppsListPreferences;
    healthBarPrefs: HealthStatusBarPreferences;
}

const ApplicationsToolbar: React.FC<ApplicationsToolbarProps> = ({pref, healthBarPrefs}) => {
    return (
        <div className='applications-list__toolbar-controls' key='app-list-tools'>
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
    useRefreshClusterList();
    const [isAppCreatePending, setAppCreatePending] = React.useState(false);
    const {List, Summary, Tiles} = AppsListViewKey;
    const authSettings = React.useContext(AuthSettingsCtx);

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

    return (
        <ClusterCtx.Provider value={true}>
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
                                    <AppsListData pref={pref} appNamespace='' hydratorEnabled={authSettings?.hydratorEnabled}>
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

                                            const {rows: filteredApps, stats} = list;

                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        key={`toolbar-${healthBarPrefs.showHealthStatusBar}-${pref.view}`}
                                                        toolbar={{
                                                            tools: <ApplicationsToolbar pref={pref} healthBarPrefs={healthBarPrefs} />,
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
                                                        <ApplicationsQueryBar
                                                            value={pref.search}
                                                            total={stats.total}
                                                            values={list.queryValues}
                                                            onSearch={value => ctx.navigation.goto('.', applicationQueryParams(value))}
                                                        />
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
                                                                                    Change the query or{' '}
                                                                                    <a onClick={() => ctx.navigation.goto('.', applicationQueryParams(''))}>clear the search</a>
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
