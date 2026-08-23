import {MockupList, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, merge, Observable} from 'rxjs';
import {bufferTime, delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {ClusterCtx, DataLoader, EmptyState, FlexTopBar, Page, Paginate, Query, SearchBar} from '../../../shared/components';
import {Consumer, ContextApis} from '../../../shared/context';
import {useObservableQuery} from '../../../shared/hooks/query';
import * as models from '../../../shared/models';
import {services, ResourcesListPreferences, HealthStatusBarPreferences, ResourcesListViewKey} from '../../../shared/services';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import * as AppUtils from '../../../applications/components/utils';
import './resources-list.scss';
import {ResourcesSummary} from './resources-summary';
import {FilteredResource, getFilterResults, ResourcesFilter} from './resources-filter';
import classNames from 'classnames';
import {isInvalidRegex} from '../../../shared/utils';
import {createMatcher} from './resources-list-search';
import {ResourcesTable} from './resources-table';
import {RESOURCE_SORT_OPTIONS, RESOURCES_LIST_SORT_KEY} from './resources-sort';
import {ResourcesStatusBar} from './resources-status-bar';
import {ResourcesDetailsPanel} from './resources-details-panel';
import {openResourceDetails} from '../utils';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

// The applications list/watch API supports only selected set of fields.
// Make sure to register any new fields in the `appFields` map of `pkg/apiclient/application/forwarder_overwrite.go`.
// The Resources view only renders the `status.resources` of each application, decorated with the
// app's name/namespace (for the owning-app link), destination cluster (`spec.destination`) and
// project (`spec.project`), both used by filters.
const APP_FIELDS = ['metadata.name', 'metadata.namespace', 'spec.destination', 'spec.project', 'status.resources'];
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

const ViewPref = ({
    children
}: {
    children: (pref: ResourcesListPreferences & {page: number; search: string; sortTitle: string; sortDirection: 'asc' | 'desc'}) => React.ReactNode;
}) => {
    const observableQuery$ = useObservableQuery();

    return (
        <DataLoader
            load={() =>
                combineLatest([services.viewPreferences.getPreferences(), observableQuery$]).pipe(
                    map(items => {
                        const preferences = items[0];
                        const params = items[1];
                        const viewPref: ResourcesListPreferences = {...preferences.resourcesList};
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
                        if (params.get('apiGroup') != null) {
                            viewPref.apiGroupFilter = params
                                .get('apiGroup')
                                .split(',')
                                .filter(item => !!item);
                        }
                        if (params.get('kind') != null) {
                            viewPref.kindFilter = params
                                .get('kind')
                                .split(',')
                                .filter(item => !!item);
                        }
                        const viewParam = params.get('view');
                        if (viewParam === ResourcesListViewKey.List || viewParam === ResourcesListViewKey.Summary) {
                            viewPref.view = viewParam;
                        }
                        const sortTitle = preferences.sortOptions?.[RESOURCES_LIST_SORT_KEY] || RESOURCE_SORT_OPTIONS[0].title;
                        const sortDirection: 'asc' | 'desc' = preferences.sortDirections?.[RESOURCES_LIST_SORT_KEY] === 'desc' ? 'desc' : 'asc';
                        return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || '', sortTitle, sortDirection};
                    })
                )
            }>
            {pref => children(pref)}
        </DataLoader>
    );
};

function filterResources(resources: models.Resource[], pref: ResourcesListPreferences, search: string): {filteredResources: models.Resource[]; filterResults: FilteredResource[]} {
    const filterResults = getFilterResults(resources, pref);
    const matches = createMatcher(search, pref.searchRegex);
    return {
        filterResults,
        filteredResources: filterResults.filter(app => matches(app.name, app.namespace) && Object.values(app.filterResult).every(val => val))
    };
}

interface ResourcesToolbarProps {
    pref: ResourcesListPreferences & {page: number; search: string};
    ctx: ContextApis;
    healthBarPrefs: HealthStatusBarPreferences;
}

const ResourcesToolbar: React.FC<ResourcesToolbarProps> = ({pref, ctx, healthBarPrefs}) => {
    const regexInvalid = pref.searchRegex && isInvalidRegex(pref.search);
    const regexToggleClass = `applications-list__regex-toggle argo-button argo-button--base${pref.searchRegex ? '' : '-o'}${
        regexInvalid ? ' applications-list__regex-toggle--invalid' : ''
    }`;

    return (
        <div className='applications-list__toolbar-controls' key='resources-list-tools'>
            <SearchBar value={pref.search} onChange={value => ctx.navigation.goto('.', {search: value || null}, {replace: true})} placeholder='Search resources...' />
            <Tooltip content={pref.searchRegex ? (regexInvalid ? 'Invalid regex pattern' : 'Regex search enabled, click to switch to plain text') : 'Click to enable regex search'}>
                <button
                    type='button'
                    aria-label='Toggle regex search'
                    aria-pressed={pref.searchRegex}
                    className={regexToggleClass}
                    onClick={() => {
                        services.viewPreferences.updatePreferences({
                            resourcesList: {...pref, searchRegex: !pref.searchRegex}
                        });
                    }}>
                    .*
                </button>
            </Tooltip>
            <Tooltip content='Toggle Health Status Bar'>
                <button
                    className={`resources-list__accordion argo-button argo-button--base${healthBarPrefs.showHealthStatusBar ? '-o' : ''}`}
                    style={{border: 'none'}}
                    onClick={() => {
                        const showHealthStatusBar = !healthBarPrefs.showHealthStatusBar;
                        services.viewPreferences.updatePreferences({
                            resourcesList: {
                                ...pref,
                                statusBarView: {
                                    ...healthBarPrefs,
                                    showHealthStatusBar
                                }
                            }
                        });
                    }}>
                    <i className={`fas fa-ruler-horizontal`} />
                </button>
            </Tooltip>
        </div>
    );
};

const ResourcesViewTypeSwitcher: React.FC<{pref: ResourcesListPreferences & {page: number; search: string}; ctx: ContextApis}> = ({pref, ctx}) => (
    <div className='applications-list__view-type'>
        <i
            className={classNames('fa fa-th-list', {selected: pref.view === ResourcesListViewKey.List}, 'menu_icon')}
            title='List'
            onClick={() => {
                ctx.navigation.goto('.', {view: ResourcesListViewKey.List});
                services.viewPreferences.updatePreferences({
                    resourcesList: {...pref, view: ResourcesListViewKey.List}
                });
            }}
        />
        <i
            className={classNames('fa fa-chart-pie', {selected: pref.view === ResourcesListViewKey.Summary}, 'menu_icon')}
            title='Summary'
            onClick={() => {
                ctx.navigation.goto('.', {view: ResourcesListViewKey.Summary});
                services.viewPreferences.updatePreferences({
                    resourcesList: {...pref, view: ResourcesListViewKey.Summary}
                });
            }}
        />
    </div>
);

export const ResourcesList = (props: RouteComponentProps<{}>) => {
    const query = new URLSearchParams(props.location.search);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const loaderRef = React.useRef<DataLoader | null>(null);

    function onFilterPrefChanged(ctx: ContextApis, newPref: ResourcesListPreferences) {
        services.viewPreferences.updatePreferences({resourcesList: newPref});
        ctx.navigation.goto(
            '.',
            {
                proj: newPref.projectsFilter.join(','),
                sync: newPref.syncFilter.join(','),
                health: newPref.healthFilter.join(','),
                namespace: newPref.namespacesFilter.join(','),
                cluster: newPref.clustersFilter.join(','),
                apiGroup: newPref.apiGroupFilter.join(','),
                kind: newPref.kindFilter.join(',')
            },
            {replace: true}
        );
    }

    function getPageTitle(view: string) {
        switch (view) {
            case ResourcesListViewKey.List:
                return 'Resources List';
            case ResourcesListViewKey.Summary:
                return 'Resources Summary';
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
                                <Page title={getPageTitle(pref.view)} useTitleOnly={true} toolbar={{breadcrumbs: [{title: 'Resources', path: '/resources'}]}}>
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
                                            const resources = applications.flatMap(app =>
                                                (app.status.resources || []).map(
                                                    item =>
                                                        ({
                                                            ...item,
                                                            appName: app.metadata.name,
                                                            appNamespace: app.metadata.namespace,
                                                            clusterServer: app.spec.destination.server,
                                                            clusterName: app.spec.destination.name,
                                                            appProject: app.spec.project
                                                        }) as models.Resource
                                                )
                                            );
                                            const {filteredResources, filterResults} = filterResources(resources, pref, pref.search);
                                            // Sorting is driven by the sortable table headers (persisted in view preferences);
                                            // apply it here so the data is already ordered before pagination.
                                            const selectedSort = RESOURCE_SORT_OPTIONS.find(option => option.title === pref.sortTitle) || RESOURCE_SORT_OPTIONS[0];
                                            const sortedResources = [...filteredResources].sort((a, b) => {
                                                const result = selectedSort.compare(a, b);
                                                return pref.sortDirection === 'asc' ? result : -result;
                                            });
                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        toolbar={{
                                                            tools: <ResourcesToolbar pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />,
                                                            options: <ResourcesViewTypeSwitcher pref={pref} ctx={ctx} />,
                                                            addAuth: true
                                                        }}
                                                    />
                                                    <div className='resources-list'>
                                                        {resources.length === 0 && pref.projectsFilter?.length === 0 ? (
                                                            <EmptyState icon='argo-icon-application'>
                                                                <h4>No resources available to you just yet</h4>
                                                                <h5>Resources from your applications will appear here once they are available</h5>
                                                            </EmptyState>
                                                        ) : (
                                                            <>
                                                                {ReactDOM.createPortal(
                                                                    <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                        {allpref => (
                                                                            <ResourcesFilter
                                                                                apps={filterResults}
                                                                                onChange={newPrefs => onFilterPrefChanged(ctx, newPrefs)}
                                                                                pref={pref}
                                                                                collapsed={allpref.hideSidebar}
                                                                            />
                                                                        )}
                                                                    </DataLoader>,
                                                                    sidebarTarget?.current
                                                                )}
                                                                {(pref.view === ResourcesListViewKey.Summary && <ResourcesSummary resources={filteredResources} />) || (
                                                                    <Paginate
                                                                        header={filteredResources.length > 1 && <ResourcesStatusBar resources={filteredResources} />}
                                                                        showHeader={healthBarPrefs.showHealthStatusBar}
                                                                        preferencesKey='resources-list'
                                                                        page={pref.page}
                                                                        emptyState={() => (
                                                                            <EmptyState icon='fa fa-search'>
                                                                                <h4>
                                                                                    {pref.searchRegex && isInvalidRegex(pref.search)
                                                                                        ? 'Invalid regex search pattern'
                                                                                        : 'No matching resources found'}
                                                                                </h4>
                                                                                <h5>
                                                                                    {pref.searchRegex && isInvalidRegex(pref.search) ? (
                                                                                        'Fix the regular expression in the search box to see matching resources'
                                                                                    ) : (
                                                                                        <>
                                                                                            Change filter criteria or&nbsp;
                                                                                            <a
                                                                                                onClick={() => {
                                                                                                    ResourcesListPreferences.clearFilters(pref);
                                                                                                    onFilterPrefChanged(ctx, pref);
                                                                                                }}>
                                                                                                clear filters
                                                                                            </a>
                                                                                        </>
                                                                                    )}
                                                                                </h5>
                                                                            </EmptyState>
                                                                        )}
                                                                        data={sortedResources}
                                                                        onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                        {data => <ResourcesTable resources={data} onOpenDetails={resource => openResourceDetails(ctx, resource)} />}
                                                                    </Paginate>
                                                                )}
                                                            </>
                                                        )}
                                                    </div>
                                                    <Query>{q => <ResourcesDetailsPanel node={q.get('node')} detailsApp={q.get('detailsApp')} />}</Query>
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
