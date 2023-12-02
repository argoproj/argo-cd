import {useData, Checkbox} from 'argo-ui/v2';
import * as minimatch from 'minimatch';
import * as React from 'react';
import {Context} from '../../../shared/context';
import {
    AbstractApplication,
    Application,
    ApplicationDestination,
    ApplicationSet,
    ApplicationSetConditionStatuses,
    ApplicationSetSpec,
    ApplicationSetStatus,
    ApplicationSpec,
    ApplicationStatus,
    Cluster,
    HealthStatusCode,
    HealthStatuses,
    Operation,
    SyncPolicy,
    SyncStatusCode,
    SyncStatuses
} from '../../../shared/models';
import {AbstractAppsListPreferences, AppSetsListPreferences, AppsListPreferences, services} from '../../../shared/services';
import {Filter, FiltersGroup} from '../filter/filter';
import * as LabelSelector from '../label-selector';
import {ComparisonStatusIcon, getAppDefaultSource, getAppSetHealthStatus, HealthStatusIcon, isApp, isInvokedFromAppsPath} from '../utils';
import {ContextApis} from '../../../shared/context';
import {History} from 'history';

export interface AbstractFilterResult {
    favourite: boolean;
    labels: boolean;
    health: boolean;
}

export interface FilterResult extends AbstractFilterResult {
    repos: boolean;
    sync: boolean;
    autosync: boolean;
    clusters: boolean;
}

export interface ApplicationSetFilterResult extends AbstractFilterResult {
    // FFU
    dummyToPlacateLinter: any;
}

export interface AbstractFilteredApp extends AbstractApplication {
    filterResult: AbstractFilterResult;
}

export interface FilteredApp extends AbstractFilteredApp {
    spec: ApplicationSpec;
    status: ApplicationStatus;
    operation?: Operation;
    isAppOfAppsPattern?: boolean;

    filterResult: FilterResult;
}

export interface ApplicationSetFilteredApp extends AbstractFilteredApp {
    spec: ApplicationSetSpec;
    status: ApplicationSetStatus;

    filterResult: ApplicationSetFilterResult;
}

function getAutoSyncStatus(syncPolicy?: SyncPolicy) {
    if (!syncPolicy || !syncPolicy.automated) {
        return 'Disabled';
    }
    return 'Enabled';
}

export function getFilterResults(applications: AbstractApplication[], pref: AbstractAppsListPreferences): AbstractFilteredApp[] {
    return applications.map(app => ({
        ...app,
        filterResult: isApp(app)
            ? {
                  repos: (pref as AppsListPreferences).reposFilter.length === 0 || (pref as AppsListPreferences).reposFilter.includes(getAppDefaultSource(app).repoURL),
                  sync: (pref as AppsListPreferences).syncFilter.length === 0 || (pref as AppsListPreferences).syncFilter.includes(app.status.sync.status),
                  autosync:
                      (pref as AppsListPreferences).autoSyncFilter.length === 0 ||
                      (pref as AppsListPreferences).autoSyncFilter.includes(getAutoSyncStatus((app as Application).spec.syncPolicy)),
                  health: pref.healthFilter.length === 0 || pref.healthFilter.includes((app as Application).status.health.status),
                  namespaces:
                      (pref as AppsListPreferences).namespacesFilter.length === 0 ||
                      (pref as AppsListPreferences).namespacesFilter.some(ns => app.spec.destination.namespace && minimatch(app.spec.destination.namespace, ns)),
                  favourite: !pref.showFavorites || (pref.favoritesAppList && pref.favoritesAppList.includes(app.metadata.name)),
                  clusters:
                      (pref as AppsListPreferences).clustersFilter.length === 0 ||
                      (pref as AppsListPreferences).clustersFilter.some(filterString => {
                          const match = filterString.match('^(.*) [(](http.*)[)]$');
                          if (match?.length === 3) {
                              const [, name, url] = match;
                              return url === app.spec.destination.server || name === app.spec.destination.name;
                          } else {
                              const inputMatch = filterString.match('^http.*$');
                              return (
                                  (inputMatch && inputMatch[0] === app.spec.destination.server) || (app.spec.destination.name && minimatch(app.spec.destination.name, filterString))
                              );
                          }
                      }),
                  labels: pref.labelsFilter.length === 0 || pref.labelsFilter.every(selector => LabelSelector.match(selector, app.metadata.labels))
              }
            : {
                  health: pref.healthFilter.length === 0 || pref.healthFilter.includes(getAppSetHealthStatus((app as ApplicationSet).status)),
                  favourite: !pref.showFavorites || (pref.favoritesAppList && pref.favoritesAppList.includes(app.metadata.name)),
                  labels: pref.labelsFilter.length === 0 || pref.labelsFilter.every(selector => LabelSelector.match(selector, app.metadata.labels))
              }
    }));
}

const optionsFrom = (options: string[], filter: string[]) => {
    return options
        .filter(s => filter.indexOf(s) === -1)
        .map(item => {
            return {label: item};
        });
};

interface AbstractAppFilterProps {
    apps: AbstractFilteredApp[]; // FilteredApp[] | ApplicationSetFilteredApp[];
    pref: AbstractAppsListPreferences;
    onChange: (newPrefs: AbstractAppsListPreferences) => void;
    children?: React.ReactNode;
    collapsed?: boolean;
}
interface AppFilterProps extends AbstractAppFilterProps {
    apps: FilteredApp[];
    pref: AppsListPreferences;
    onChange: (newPrefs: AppsListPreferences) => void;
}

interface ApplicationSetFilterProps extends AbstractAppFilterProps {
    apps: ApplicationSetFilteredApp[];
    pref: AppSetsListPreferences;
    onChange: (newPrefs: AppSetsListPreferences) => void;
}
export function isAppFilterProps(
    abstractAppFilterProps: AbstractAppFilterProps,
    ctx: ContextApis & {
        history: History<unknown>;
    }
): abstractAppFilterProps is AppFilterProps {
    // return isApp(abstractAppFilterProps.apps[0]);
    return isInvokedFromAppsPath(ctx.history.location.pathname);
}

const getCounts = (apps: FilteredApp[], filterType: keyof FilterResult, filter: (app: Application) => string, init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    // filter out all apps that does not match other filters and ignore this filter result
    apps.filter(app => filter(app) && Object.keys(app.filterResult).every((key: keyof FilterResult) => key === filterType || app.filterResult[key])).forEach(app =>
        map.set(filter(app), (map.get(filter(app)) || 0) + 1)
    );
    return map;
};

const getAppSetCounts = (apps: ApplicationSetFilteredApp[], filterType: keyof ApplicationSetFilterResult, filter: (app: ApplicationSet) => string, init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    // filter out all apps that does not match other filters and ignore this filter result
    apps.filter(app => filter(app) && Object.keys(app.filterResult).every((key: keyof ApplicationSetFilterResult) => key === filterType || app.filterResult[key])).forEach(app =>
        map.set(filter(app), (map.get(filter(app)) || 0) + 1)
    );
    return map;
};

const getOptions = (apps: FilteredApp[], filterType: keyof FilterResult, filter: (app: Application) => string, keys: string[], getIcon?: (k: string) => React.ReactNode) => {
    const counts = getCounts(apps, filterType, filter, keys);
    return keys.map(k => {
        return {
            label: k,
            icon: getIcon && getIcon(k),
            count: counts.get(k)
        };
    });
};

const getAppSetOptions = (
    apps: ApplicationSetFilteredApp[],
    filterType: keyof ApplicationSetFilterResult,
    filter: (app: ApplicationSet) => string,
    keys: string[],
    getIcon?: (k: string) => React.ReactNode
) => {
    const counts = getAppSetCounts(apps, filterType, filter, keys);
    return keys.map(k => {
        return {
            label: k,
            icon: getIcon && getIcon(k),
            count: counts.get(k)
        };
    });
};

const SyncFilter = (props: AppFilterProps) => (
    <Filter
        label='SYNC STATUS'
        selected={props.pref.syncFilter}
        setSelected={s => props.onChange({...props.pref, syncFilter: s})}
        options={getOptions(
            props.apps,
            'sync',
            app => app.status.sync.status,
            Object.keys(SyncStatuses),
            s => (
                <ComparisonStatusIcon status={s as SyncStatusCode} noSpin={true} />
            )
        )}
    />
);

const HealthFilter = (props: AbstractAppFilterProps) => {
    const ctx = React.useContext(Context);
    return (
        <Filter
            label='HEALTH STATUS'
            selected={props.pref.healthFilter}
            setSelected={s =>
                isAppFilterProps(props, ctx) ? props.onChange({...props.pref, healthFilter: s}) : props.onChange({...(props as ApplicationSetFilterProps).pref, healthFilter: s})
            }
            options={
                isAppFilterProps(props, ctx)
                    ? getOptions(
                          props.apps,
                          'health',
                          app => app.status.health.status,
                          Object.keys(HealthStatuses),
                          s => <HealthStatusIcon state={{status: s as HealthStatusCode, message: ''}} noSpin={true} />
                      )
                    : getAppSetOptions(
                          (props as ApplicationSetFilterProps).apps,
                          'health',
                          app => getAppSetHealthStatus(app.status),
                          Object.keys(ApplicationSetConditionStatuses)
                          // s => (
                          //     <HealthStatusIcon state={{ status: s as HealthStatusCode, message: '' }} noSpin={true} />
                          // )
                      )
            }
        />
    );
};

const LabelsFilter = (props: AbstractAppFilterProps) => {
    const labels = new Map<string, Set<string>>();
    (props.apps as AbstractFilteredApp[])
        .filter(app => app.metadata && app.metadata.labels)
        .forEach(app =>
            Object.keys(app.metadata.labels).forEach(label => {
                let values = labels.get(label);
                if (!values) {
                    values = new Set<string>();
                    labels.set(label, values);
                }
                values.add(app.metadata.labels[label]);
            })
        );
    const suggestions = new Array<string>();
    Array.from(labels.entries()).forEach(([label, values]) => {
        suggestions.push(label);
        values.forEach(val => suggestions.push(`${label}=${val}`));
    });
    const labelOptions = suggestions.map(s => {
        return {label: s};
    });

    return <Filter label='LABELS' selected={props.pref.labelsFilter} setSelected={s => props.onChange({...props.pref, labelsFilter: s})} field={true} options={labelOptions} />;
};

const ProjectFilter = (props: AppFilterProps) => {
    const [projects, loading, error] = useData(
        () => services.projects.list('items.metadata.name'),
        null,
        () => null
    );
    const projectOptions = (projects || []).map(proj => {
        return {label: proj.metadata.name};
    });
    return (
        <Filter
            label='PROJECTS'
            selected={props.pref.projectsFilter}
            setSelected={s => props.onChange({...props.pref, projectsFilter: s})}
            field={true}
            options={projectOptions}
            error={error.state}
            retry={error.retry}
            loading={loading}
        />
    );
};

const ClusterFilter = (props: AppFilterProps) => {
    const getClusterDetail = (dest: ApplicationDestination, clusterList: Cluster[]): string => {
        const cluster = (clusterList || []).find(target => target.name === dest.name || target.server === dest.server);
        if (!cluster) {
            return dest.server || dest.name;
        }
        if (cluster.name === cluster.server) {
            return cluster.name;
        }
        return `${cluster.name} (${cluster.server})`;
    };

    const [clusters, loading, error] = useData(() => services.clusters.list());
    const clusterOptions = optionsFrom(
        Array.from(new Set(props.apps.map(app => getClusterDetail(app.spec.destination, clusters)).filter(item => !!item))),
        props.pref.clustersFilter
    );

    return (
        <Filter
            label='CLUSTERS'
            selected={props.pref.clustersFilter}
            setSelected={s => props.onChange({...props.pref, clustersFilter: s})}
            field={true}
            options={clusterOptions}
            error={error.state}
            retry={error.retry}
            loading={loading}
        />
    );
};

const NamespaceFilter = (props: AppFilterProps) => {
    const namespaceOptions = optionsFrom(Array.from(new Set(props.apps.map(app => app.spec.destination.namespace).filter(item => !!item))), props.pref.namespacesFilter);
    return (
        <Filter
            label='NAMESPACES'
            selected={props.pref.namespacesFilter}
            setSelected={s => props.onChange({...props.pref, namespacesFilter: s})}
            field={true}
            options={namespaceOptions}
        />
    );
};

const FavoriteFilter = (props: AbstractAppFilterProps) => {
    const ctx = React.useContext(Context);
    const onChange = (val: boolean) => {
        ctx.navigation.goto('.', {showFavorites: val}, {replace: true});
        services.viewPreferences.updatePreferences({appList: {...props.pref, showFavorites: val}});
    };
    return (
        <div
            className={`filter filter__item ${props.pref.showFavorites ? 'filter__item--selected' : ''}`}
            style={{margin: '0.5em 0', marginTop: '0.5em'}}
            onClick={() => onChange(!props.pref.showFavorites)}>
            <Checkbox
                value={!!props.pref.showFavorites}
                onChange={onChange}
                style={{
                    marginRight: '8px'
                }}
            />
            <div style={{marginRight: '5px', textAlign: 'center', width: '25px'}}>
                <i style={{color: '#FFCE25'}} className='fas fa-star' />
            </div>
            <div className='filter__item__label'>Favorites Only</div>
        </div>
    );
};

function getAutoSyncOptions(apps: FilteredApp[]) {
    const counts = getCounts(apps, 'autosync', app => getAutoSyncStatus(app.spec.syncPolicy), ['Enabled', 'Disabled']);
    return [
        {
            label: 'Enabled',
            icon: <i className='fa fa-circle-play' />,
            count: counts.get('Enabled')
        },
        {
            label: 'Disabled',
            icon: <i className='fa fa-ban' />,
            count: counts.get('Disabled')
        }
    ];
}

const AutoSyncFilter = (props: AppFilterProps) => (
    <Filter
        label='AUTO SYNC'
        selected={props.pref.autoSyncFilter}
        setSelected={s => props.onChange({...props.pref, autoSyncFilter: s})}
        options={getAutoSyncOptions(props.apps)}
        collapsed={props.collapsed || false}
    />
);

export const ApplicationsFilter = (props: AbstractAppFilterProps) => {
    const ctx = React.useContext(Context);
    return (
        <FiltersGroup content={props.children} collapsed={props.collapsed}>
            <FavoriteFilter {...props} />
            {isAppFilterProps(props, ctx) && <SyncFilter {...props} />}
            <HealthFilter {...props} />
            <LabelsFilter {...props} />
            {isAppFilterProps(props, ctx) && <ProjectFilter {...props} />}
            {isAppFilterProps(props, ctx) && <ClusterFilter {...props} />}
            {isAppFilterProps(props, ctx) && <NamespaceFilter {...props} />}
            {isAppFilterProps(props, ctx) && <AutoSyncFilter {...props} collapsed={true} />}
        </FiltersGroup>
    );
};
