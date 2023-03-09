import {useData, Checkbox} from 'argo-ui/v2';
import * as minimatch from 'minimatch';
import * as React from 'react';
import {Context} from '../../../shared/context';
import {Application, ApplicationDestination, Cluster, HealthStatusCode, HealthStatuses, SyncPolicy, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import {Filter, FiltersGroup} from '../filter/filter';
import * as LabelSelector from '../label-selector';
import {ComparisonStatusIcon, getAppDefaultSource, HealthStatusIcon} from '../utils';

export interface FilterResult {
    repos: boolean;
    sync: boolean;
    autosync: boolean;
    health: boolean;
    namespaces: boolean;
    clusters: boolean;
    favourite: boolean;
    labels: boolean;
}

export interface FilteredApp extends Application {
    filterResult: FilterResult;
}

function getAutoSyncStatus(syncPolicy?: SyncPolicy) {
    if (!syncPolicy || !syncPolicy.automated) {
        return 'Disabled';
    }
    return 'Enabled';
}

export function getFilterResults(applications: Application[], pref: AppsListPreferences): FilteredApp[] {
    return applications.map(app => ({
        ...app,
        filterResult: {
            repos: pref.reposFilter.length === 0 || pref.reposFilter.includes(getAppDefaultSource(app).repoURL),
            sync: pref.syncFilter.length === 0 || pref.syncFilter.includes(app.status.sync.status),
            autosync: pref.autoSyncFilter.length === 0 || pref.autoSyncFilter.includes(getAutoSyncStatus(app.spec.syncPolicy)),
            health: pref.healthFilter.length === 0 || pref.healthFilter.includes(app.status.health.status),
            namespaces: pref.namespacesFilter.length === 0 || pref.namespacesFilter.some(ns => app.spec.destination.namespace && minimatch(app.spec.destination.namespace, ns)),
            favourite: !pref.showFavorites || (pref.favoritesAppList && pref.favoritesAppList.includes(app.metadata.name)),
            clusters:
                pref.clustersFilter.length === 0 ||
                pref.clustersFilter.some(filterString => {
                    const match = filterString.match('^(.*) [(](http.*)[)]$');
                    if (match?.length === 3) {
                        const [, name, url] = match;
                        return url === app.spec.destination.server || name === app.spec.destination.name;
                    } else {
                        const inputMatch = filterString.match('^http.*$');
                        return (inputMatch && inputMatch[0] === app.spec.destination.server) || (app.spec.destination.name && minimatch(app.spec.destination.name, filterString));
                    }
                }),
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

interface AppFilterProps {
    apps: FilteredApp[];
    pref: AppsListPreferences;
    onChange: (newPrefs: AppsListPreferences) => void;
    children?: React.ReactNode;
    collapsed?: boolean;
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

const HealthFilter = (props: AppFilterProps) => (
    <Filter
        label='HEALTH STATUS'
        selected={props.pref.healthFilter}
        setSelected={s => props.onChange({...props.pref, healthFilter: s})}
        options={getOptions(
            props.apps,
            'health',
            app => app.status.health.status,
            Object.keys(HealthStatuses),
            s => (
                <HealthStatusIcon state={{status: s as HealthStatusCode, message: ''}} noSpin={true} />
            )
        )}
    />
);

const LabelsFilter = (props: AppFilterProps) => {
    const labels = new Map<string, Set<string>>();
    props.apps
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

const FavoriteFilter = (props: AppFilterProps) => {
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

export const ApplicationsFilter = (props: AppFilterProps) => {
    return (
        <FiltersGroup content={props.children} collapsed={props.collapsed}>
            <FavoriteFilter {...props} />
            <SyncFilter {...props} />
            <HealthFilter {...props} />
            <LabelsFilter {...props} />
            <ProjectFilter {...props} />
            <ClusterFilter {...props} />
            <NamespaceFilter {...props} />
            <AutoSyncFilter {...props} collapsed={true} />
        </FiltersGroup>
    );
};
