import {Autocomplete, MockupList, Toolbar, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Key, KeybindingContext, KeybindingProvider, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, merge, Observable} from 'rxjs';
import {bufferTime, delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {AddAuthToToolbar, DataLoader, EmptyState, Page, Paginate} from '../../../shared/components';
import {AuthSettingsCtx, Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppsListPreferences, AppsListViewKey, AppsListViewType, AppSetsListPreferences, HealthStatusBarPreferences, services, ViewPreferences} from '../../../shared/services';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {useObservableQuery, useQuery} from '../../../shared/hooks/query';
import * as AppUtils from '../utils';
import {AppSetsFilter, ApplicationSetFilteredApp, getAppSetFilterResults} from './applications-filter';
import {AppSetsStatusBar} from './applications-status-bar';
import {AppSetTile} from './appset-tile';
import {AppSetTableRow} from './appset-table-row';
import {ApplicationSetsSummary} from './application-sets-summary';

import './applications-list.scss';
import './applications-table.scss';
import './applications-tiles.scss';
import './flex-top-bar.scss';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

const APPSET_FIELDS = [
    'metadata.name',
    'metadata.namespace',
    'metadata.annotations',
    'metadata.labels',
    'metadata.creationTimestamp',
    'metadata.deletionTimestamp',
    'spec',
    'status.conditions',
    'status.resources',
    'status.resourcesCount',
    'status.health'
];
const APPSET_LIST_FIELDS = ['metadata.resourceVersion', ...APPSET_FIELDS.map(field => `items.${field}`)];
const APPSET_WATCH_FIELDS = ['result.type', ...APPSET_FIELDS.map(field => `result.applicationSet.${field}`)];

function loadApplicationSets(projects: string[]): Observable<models.ApplicationSet[]> {
    return from(services.applications.list(projects, 'applicationset', {fields: APPSET_LIST_FIELDS})).pipe(
        mergeMap(applicationsList => {
            const appSets = applicationsList.items as models.ApplicationSet[];
            return merge(
                from([appSets]),
                services.applications
                    .watch('applicationset', {projects, resourceVersion: applicationsList.metadata.resourceVersion}, {fields: APPSET_WATCH_FIELDS})
                    .pipe(repeat())
                    .pipe(retryWhen(errors => errors.pipe(delay(WATCH_RETRY_TIMEOUT))))
                    .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
                    .pipe(
                        map(appChanges => {
                            appChanges.forEach(appChange => {
                                const appSet = appChange.application as unknown as models.ApplicationSet;
                                const index = appSets.findIndex(item => AppUtils.appInstanceName(item) === AppUtils.appInstanceName(appSet));
                                switch (appChange.type) {
                                    case 'DELETED':
                                        if (index > -1) {
                                            appSets.splice(index, 1);
                                        }
                                        break;
                                    default:
                                        if (index > -1) {
                                            appSets[index] = appSet;
                                        } else {
                                            appSets.unshift(appSet);
                                        }
                                        break;
                                }
                            });
                            return {appSets, updated: appChanges.length > 0};
                        })
                    )
                    .pipe(filter(item => item.updated))
                    .pipe(map(item => item.appSets))
            );
        })
    );
}

const ViewPref = ({children}: {children: (pref: AppsListPreferences & {page: number; search: string}) => React.ReactNode}) => {
    const observableQuery$ = useObservableQuery();

    return (
        <DataLoader
            load={() =>
                combineLatest([services.viewPreferences.getPreferences().pipe(map(item => item.appList)), observableQuery$]).pipe(
                    map(items => {
                        const params = items[1];
                        const viewPref: AppsListPreferences = {...items[0]};
                        if (params.get('health') != null) {
                            viewPref.healthFilter = params
                                .get('health')
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
                        return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || ''};
                    })
                )
            }>
            {pref => children(pref)}
        </DataLoader>
    );
};

function filterApplicationSets(
    appSets: models.ApplicationSet[],
    pref: AppSetsListPreferences,
    search: string
): {filteredApps: models.ApplicationSet[]; filterResults: ApplicationSetFilteredApp[]} {
    const filterResults = getAppSetFilterResults(appSets, pref);

    return {
        filterResults,
        filteredApps: filterResults.filter(
            app => (search === '' || app.metadata.name.includes(search) || app.metadata.namespace.includes(search)) && Object.values(app.filterResult).every(val => val)
        )
    };
}

const ApplicationSetsSearchBar = (props: {content: string; ctx: ContextApis; appSets: models.ApplicationSet[]}) => {
    const searchBar = React.useRef<HTMLDivElement>(null);
    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (searchBar.current) {
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
            if (searchBar.current && isFocused) {
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
                <div className='applications-list__search' ref={searchBar}>
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
                        placeholder='Search application sets...'
                    />
                    <div className='keyboard-hint'>/</div>
                    {props.content && (
                        <i className='fa fa-times' onClick={() => props.ctx.navigation.goto('.', {search: null}, {replace: true})} style={{cursor: 'pointer', marginLeft: '5px'}} />
                    )}
                </div>
            )}
            wrapperProps={{className: 'applications-list__search-wrapper'}}
            renderItem={item => (
                <React.Fragment>
                    <i className='icon argo-icon-applicationset' /> {item.label}
                </React.Fragment>
            )}
            onSelect={val => {
                const selectedAppSet = props.appSets?.find(appSet => {
                    const qualifiedName = AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);
                    return qualifiedName === val;
                });
                if (selectedAppSet) {
                    props.ctx.navigation.goto(`/${AppUtils.getAppUrl(selectedAppSet)}`);
                }
            }}
            onChange={e => props.ctx.navigation.goto('.', {search: e.target.value}, {replace: true})}
            value={props.content || ''}
            items={props.appSets.map(appSet => AppUtils.appQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled))}
        />
    );
};

const ApplicationSetsToolbar = (props: {
    appSets: models.ApplicationSet[];
    pref: AppsListPreferences & {page: number; search: string};
    ctx: ContextApis;
    healthBarPrefs: HealthStatusBarPreferences;
}) => {
    const {List, Summary, Tiles} = AppsListViewKey;
    const query = useQuery();

    return (
        <React.Fragment key='appset-list-tools'>
            <ApplicationSetsSearchBar content={query.get('search')} appSets={props.appSets} ctx={props.ctx} />
            <Tooltip content='Toggle Health Status Bar'>
                <button
                    className={`applications-list__accordion argo-button argo-button--base${props.healthBarPrefs.showHealthStatusBar ? '-o' : ''}`}
                    style={{border: 'none'}}
                    onClick={() => {
                        props.healthBarPrefs.showHealthStatusBar = !props.healthBarPrefs.showHealthStatusBar;
                        services.viewPreferences.updatePreferences({
                            appList: {
                                ...props.pref,
                                statusBarView: {
                                    ...props.healthBarPrefs,
                                    showHealthStatusBar: props.healthBarPrefs.showHealthStatusBar
                                }
                            }
                        });
                    }}>
                    <i className='fas fa-ruler-horizontal' />
                </button>
            </Tooltip>
            <div className='applications-list__view-type' style={{marginLeft: 'auto'}}>
                <i
                    className={classNames('fa fa-th', {selected: props.pref.view === Tiles}, 'menu_icon')}
                    title='Tiles'
                    onClick={() => {
                        props.ctx.navigation.goto('.', {view: Tiles});
                        services.viewPreferences.updatePreferences({appList: {...props.pref, view: Tiles}});
                    }}
                />
                <i
                    className={classNames('fa fa-th-list', {selected: props.pref.view === List}, 'menu_icon')}
                    title='List'
                    onClick={() => {
                        props.ctx.navigation.goto('.', {view: List});
                        services.viewPreferences.updatePreferences({appList: {...props.pref, view: List}});
                    }}
                />
                <i
                    className={classNames('fa fa-chart-pie', {selected: props.pref.view === Summary}, 'menu_icon')}
                    title='Summary'
                    onClick={() => {
                        props.ctx.navigation.goto('.', {view: Summary});
                        services.viewPreferences.updatePreferences({appList: {...props.pref, view: Summary}});
                    }}
                />
            </div>
        </React.Fragment>
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
                            <div className='flex-top-bar__actions'>
                                {toolbar.actionMenu && (
                                    <React.Fragment>
                                        {toolbar.actionMenu.items.map((item, i) => (
                                            <Tooltip className='custom-tooltip' content={item.title} key={item.qeId || i}>
                                                <button
                                                    disabled={!!item.disabled}
                                                    qe-id={item.qeId}
                                                    className='argo-button argo-button--base'
                                                    onClick={() => item.action()}
                                                    style={{marginRight: 2}}
                                                    key={i}>
                                                    {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                                                    <span className='show-for-large'>{item.title}</span>
                                                </button>
                                            </Tooltip>
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

const useItemsPerContainer = (itemRef: any, containerRef: any): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        const handleResize = () => {
            let timeoutId: any;
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                timeoutId = null;
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                if (curItemsPer !== itemsPer) {
                    setItemsPer(curItemsPer);
                }
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    return itemsPer || 1;
};

const ApplicationSetTiles = ({appSets}: {appSets: models.ApplicationSet[]}) => {
    const [selectedAppSet, navAppSet, reset] = useNav(appSets.length);
    const ctxh = React.useContext(Context);
    const firstTileRef = React.useRef<HTMLDivElement>(null);
    const appSetContainerRef = React.useRef(null);
    const appSetsPerRow = useItemsPerContainer(firstTileRef, appSetContainerRef);
    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.RIGHT, action: () => navAppSet(1)});
    useKeybinding({keys: Key.LEFT, action: () => navAppSet(-1)});
    useKeybinding({keys: Key.DOWN, action: () => navAppSet(appSetsPerRow)});
    useKeybinding({keys: Key.UP, action: () => navAppSet(-1 * appSetsPerRow)});
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedAppSet > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(appSets[selectedAppSet])}`);
                return true;
            }
            return false;
        }
    });
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedAppSet > -1) {
                reset();
                return true;
            }
            return false;
        }
    });
    useKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navAppSet(NumKeyToNumber(n));
        }
    });
    useKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navAppSet(NumKeyToNumber(n));
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {(pref: ViewPreferences) => (
                        <div className='applications-tiles argo-table-list argo-table-list--clickable' ref={appSetContainerRef}>
                            {appSets.map((appSet, i) => (
                                <AppSetTile
                                    key={AppUtils.appInstanceName(appSet)}
                                    appSet={appSet}
                                    selected={selectedAppSet === i}
                                    pref={pref}
                                    ctx={ctx}
                                    tileRef={i === 0 ? firstTileRef : undefined}
                                />
                            ))}
                        </div>
                    )}
                </DataLoader>
            )}
        </Consumer>
    );
};

const ApplicationSetTable = ({appSets}: {appSets: models.ApplicationSet[]}) => {
    const [selectedAppSet, navAppSet, reset] = useNav(appSets.length);
    const ctxh = React.useContext(Context);
    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.DOWN, action: () => navAppSet(1)});
    useKeybinding({keys: Key.UP, action: () => navAppSet(-1)});
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedAppSet > -1 ? true : false;
        }
    });
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedAppSet > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(appSets[selectedAppSet])}`);
                return true;
            }
            return false;
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {(pref: ViewPreferences) => (
                        <div className='applications-table argo-table-list argo-table-list--clickable'>
                            {appSets.map((appSet, i) => (
                                <AppSetTableRow key={AppUtils.appInstanceName(appSet)} appSet={appSet} selected={selectedAppSet === i} pref={pref} ctx={ctx} />
                            ))}
                        </div>
                    )}
                </DataLoader>
            )}
        </Consumer>
    );
};

export const ApplicationSetsList = (props: RouteComponentProps<any>) => {
    const {List, Summary, Tiles} = AppsListViewKey;
    const sidebarTarget = useSidebarTarget();

    function onAppSetFilterPrefChanged(ctx: ContextApis, newPref: AppSetsListPreferences) {
        services.viewPreferences.updatePreferences({appList: newPref as AppsListPreferences});
        ctx.navigation.goto(
            '.',
            {
                health: newPref.healthFilter.join(','),
                labels: newPref.labelsFilter.map(encodeURIComponent).join(','),
                showFavorites: newPref.showFavorites ? 'true' : null
            },
            {replace: true}
        );
    }

    function getPageTitle(view: string) {
        switch (view) {
            case List:
                return 'ApplicationSets List';
            case Tiles:
                return 'ApplicationSets Tiles';
            case Summary:
                return 'ApplicationSets Summary';
        }
        return '';
    }

    return (
        <KeybindingProvider>
            <Consumer>
                {ctx => (
                    <ViewPref>
                        {pref => (
                            <Page
                                key={pref.view}
                                title={getPageTitle(pref.view)}
                                useTitleOnly={true}
                                toolbar={{breadcrumbs: [{title: 'ApplicationSets', path: props.match.url}]}}
                                hideAuth={true}>
                                <DataLoader
                                    input={pref.projectsFilter?.join(',')}
                                    load={() => AppUtils.handlePageVisibility(() => loadApplicationSets(pref.projectsFilter))}
                                    loadingRenderer={() => (
                                        <div className='argo-container'>
                                            <MockupList height={100} marginTop={30} />
                                        </div>
                                    )}>
                                    {(appSets: models.ApplicationSet[]) => {
                                        const healthBarPrefs = pref.statusBarView || ({} as HealthStatusBarPreferences);
                                        const appSetPref: AppSetsListPreferences = {
                                            labelsFilter: pref.labelsFilter,
                                            healthFilter: pref.healthFilter,
                                            showFavorites: pref.showFavorites,
                                            favoritesAppList: pref.favoritesAppList,
                                            view: pref.view,
                                            hideFilters: pref.hideFilters,
                                            statusBarView: pref.statusBarView,
                                            annotationsFilter: pref.annotationsFilter
                                        };
                                        const {filteredApps, filterResults} = filterApplicationSets(appSets, appSetPref, pref.search);

                                        return (
                                            <React.Fragment>
                                                <FlexTopBar
                                                    toolbar={{
                                                        tools: <ApplicationSetsToolbar appSets={appSets} pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />,
                                                        actionMenu: {
                                                            items: []
                                                        }
                                                    }}
                                                />
                                                <div className='applications-list'>
                                                    {appSets.length === 0 && (pref.labelsFilter || []).length === 0 ? (
                                                        <EmptyState icon='argo-icon-applicationset'>
                                                            <h4>No ApplicationSets available to you just yet</h4>
                                                            <h5>ApplicationSets will appear here once created</h5>
                                                        </EmptyState>
                                                    ) : (
                                                        <>
                                                            {ReactDOM.createPortal(
                                                                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                    {allpref => (
                                                                        <AppSetsFilter
                                                                            apps={filterResults}
                                                                            onChange={newPrefs => onAppSetFilterPrefChanged(ctx, newPrefs)}
                                                                            pref={appSetPref}
                                                                            collapsed={allpref.hideSidebar}
                                                                        />
                                                                    )}
                                                                </DataLoader>,
                                                                sidebarTarget?.current
                                                            )}

                                                            {pref.view === Summary ? (
                                                                <ApplicationSetsSummary appSets={filteredApps} />
                                                            ) : (
                                                                <Paginate
                                                                    header={filteredApps.length > 1 && <AppSetsStatusBar appSets={filteredApps} />}
                                                                    showHeader={healthBarPrefs.showHealthStatusBar}
                                                                    preferencesKey='applications-list'
                                                                    page={pref.page}
                                                                    emptyState={() => (
                                                                        <EmptyState icon='fa fa-search'>
                                                                            <h4>No matching application sets found</h4>
                                                                            <h5>
                                                                                Change filter criteria or&nbsp;
                                                                                <a
                                                                                    onClick={() => {
                                                                                        AppSetsListPreferences.clearFilters(appSetPref);
                                                                                        onAppSetFilterPrefChanged(ctx, appSetPref);
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
                                                                        }
                                                                    ]}
                                                                    data={filteredApps}
                                                                    onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                    {data =>
                                                                        (pref.view === Tiles && <ApplicationSetTiles appSets={data} />) || <ApplicationSetTable appSets={data} />
                                                                    }
                                                                </Paginate>
                                                            )}
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
    );
};
