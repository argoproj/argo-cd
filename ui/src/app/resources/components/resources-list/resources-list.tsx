import {MockupList, Toolbar} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Key, KeybindingContext, KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, merge, Observable} from 'rxjs';
import {bufferTime, delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {AddAuthToToolbar, ClusterCtx, DataLoader, EmptyState, ObservableQuery, Page, Paginate, Query} from '../../../shared/components';
import {Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services, ResourcesListPreferences} from '../../../shared/services';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import * as AppUtils from '../../../applications/components/utils';
import './resources-list.scss';
import './flex-top-bar.scss';
import {ResourceTiles} from './resources-tiles';
import {FilteredResource, getFilterResults, ResourcesFilter} from './resources-filter';

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
    'spec',
    'operation.sync',
    'status.sync.status',
    'status.sync.revision',
    'status.health',
    'status.operationState.phase',
    'status.operationState.finishedAt',
    'status.operationState.operation.sync',
    'status.summary',
    'status.resources'
];
const APP_LIST_FIELDS = ['metadata.resourceVersion', ...APP_FIELDS.map(field => `items.${field}`)];
const APP_WATCH_FIELDS = ['result.type', ...APP_FIELDS.map(field => `result.application.${field}`)];

function loadApplications(projects: string[], appNamespace: string): Observable<models.Application[]> {
    return from(services.applications.list(projects, {appNamespace, fields: APP_LIST_FIELDS})).pipe(
        mergeMap(applicationsList => {
            const applications = applicationsList.items;
            return merge(
                from([applications]),
                services.applications
                    .watch({projects, resourceVersion: applicationsList.metadata.resourceVersion}, {fields: APP_WATCH_FIELDS})
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

const ViewPref = ({children}: {children: (pref: ResourcesListPreferences & {page: number; search: string}) => React.ReactNode}) => (
    <ObservableQuery>
        {q => (
            <DataLoader
                load={() =>
                    combineLatest([services.viewPreferences.getPreferences().pipe(map(item => item.resourcesList)), q]).pipe(
                        map(items => {
                            const params = items[1];
                            const viewPref: ResourcesListPreferences = {...items[0]};
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
                            return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || ''};
                        })
                    )
                }>
                {pref => children(pref)}
            </DataLoader>
        )}
    </ObservableQuery>
);

function filterResources(resources: models.Resource[], pref: ResourcesListPreferences, search: string): {filteredResources: models.Resource[]; filterResults: FilteredResource[]} {
    const filterResults = getFilterResults(resources, pref);
    return {
        filterResults,
        filteredResources: filterResults.filter(
            app =>
                (search === '' || app.name?.includes(search) || app.kind?.includes(search) || app.group?.includes(search) || app.namespace?.includes(search)) &&
                Object.values(app.filterResult).every(val => val)
        )
    };
}

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

const SearchBar = (props: {content: string; ctx: ContextApis; resources: models.Resource[]}) => {
    const {content, ctx} = {...props};

    const searchBar = React.useRef<HTMLDivElement>(null);

    const query = new URLSearchParams(window.location.search);
    const appInput = tryJsonParse(query.get('new'));

    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);

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
        <div className='resources-list__search-wrapper'>
            <div className='resources-list__search' ref={searchBar}>
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
                    onFocus={e => {
                        e.target.select();
                    }}
                    onChange={e => ctx.navigation.goto('.', {search: e.target.value}, {replace: true})}
                    value={content || ''}
                    style={{fontSize: '14px'}}
                    className='argo-field'
                    placeholder='Search resources...'
                />
                <div className='keyboard-hint'>/</div>
                {content && <i className='fa fa-times' onClick={() => ctx.navigation.goto('.', {search: null}, {replace: true})} style={{cursor: 'pointer', marginLeft: '5px'}} />}
            </div>
        </div>
    );
};

const FlexTopBar = (props: {toolbar: Toolbar | Observable<Toolbar>}) => {
    const ctx = React.useContext(Context);
    const loadToolbar = AddAuthToToolbar(props.toolbar, ctx);
    return (
        <React.Fragment>
            <div className='top-bar row flex-top-bar' key='tool-bar'>
                <DataLoader load={() => loadToolbar}>
                    {toolbar => (
                        <React.Fragment>
                            <div className='flex-top-bar__tools'>{toolbar.tools}</div>
                        </React.Fragment>
                    )}
                </DataLoader>
            </div>
            <div className='flex-top-bar__padder' />
        </React.Fragment>
    );
};

export const ResourcesList = (props: RouteComponentProps<{}>) => {
    const query = new URLSearchParams(props.location.search);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const loaderRef = React.useRef<DataLoader>();

    function onFilterPrefChanged(ctx: ContextApis, newPref: ResourcesListPreferences) {
        services.viewPreferences.updatePreferences({resourcesList: newPref});
        ctx.navigation.goto(
            '.',
            {
                proj: newPref.projectsFilter.join(','),
                sync: newPref.syncFilter.join(','),
                health: newPref.healthFilter.join(','),
                namespace: newPref.namespacesFilter.join(','),
                cluster: newPref.clustersFilter.join(',')
            },
            {replace: true}
        );
    }

    const sidebarTarget = useSidebarTarget();

    return (
        <ClusterCtx.Provider value={clusters}>
            <KeybindingProvider>
                <Consumer>
                    {ctx => (
                        <ViewPref>
                            {pref => (
                                <Page title={'Resource List'} useTitleOnly={true} toolbar={{breadcrumbs: [{title: 'Resources', path: '/resources'}]}} hideAuth={true}>
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
                                            const resources = applications
                                                .map(app =>
                                                    app.status.resources.map(
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
                                                )
                                                .flat();
                                            const {filteredResources, filterResults} = filterResources(resources, pref, pref.search);
                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        toolbar={{
                                                            tools: (
                                                                <React.Fragment key='app-list-tools'>
                                                                    <Query>{q => <SearchBar content={q.get('search')} resources={resources} ctx={ctx} />}</Query>
                                                                </React.Fragment>
                                                            )
                                                        }}
                                                    />
                                                    <div className='resources-list'>
                                                        {resources.length === 0 && pref.projectsFilter?.length === 0 ? (
                                                            <EmptyState icon='argo-icon-application'>
                                                                <h4>No resources available to you just yet</h4>
                                                                <h5>Create new application to start managing resources in your cluster</h5>
                                                                <button
                                                                    qe-id='resources-list-button-create-application'
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
                                                                <Paginate
                                                                    preferencesKey='resources-list'
                                                                    page={pref.page}
                                                                    emptyState={() => (
                                                                        <EmptyState icon='fa fa-search'>
                                                                            <h4>No matching applications found</h4>
                                                                            <h5>
                                                                                Change filter criteria or&nbsp;
                                                                                <a
                                                                                    onClick={() => {
                                                                                        ResourcesListPreferences.clearFilters(pref);
                                                                                        onFilterPrefChanged(ctx, pref);
                                                                                    }}>
                                                                                    clear filters
                                                                                </a>
                                                                            </h5>
                                                                        </EmptyState>
                                                                    )}
                                                                    sortOptions={[{title: 'Name', compare: (a, b) => a.name.localeCompare(b.name)}]}
                                                                    data={filteredResources}
                                                                    onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                    {data => <ResourceTiles resources={data} />}
                                                                </Paginate>
                                                            </>
                                                        )}
                                                    </div>
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
