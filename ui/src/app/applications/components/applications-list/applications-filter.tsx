import {ActionButton, debounce, useData} from 'argo-ui/v2';
import * as minimatch from 'minimatch';
import * as React from 'react';
import {Application, ApplicationDestination, Cluster, HealthStatusCode, HealthStatuses, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import {Filter} from '../filter/filter';
import * as LabelSelector from '../label-selector';
import {ComparisonStatusIcon, HealthStatusIcon} from '../utils';

export interface FilterResult {
    projects: boolean;
    repos: boolean;
    sync: boolean;
    health: boolean;
    namespaces: boolean;
    clusters: boolean;
    labels: boolean;
}

export interface FilteredApp extends Application {
    filterResult: FilterResult;
}

export function getFilterResults(applications: Application[], pref: AppsListPreferences): FilteredApp[] {
    return applications.map(app => ({
        ...app,
        filterResult: {
            projects: pref.projectsFilter.length === 0 || pref.projectsFilter.includes(app.spec.project),
            repos: pref.reposFilter.length === 0 || pref.reposFilter.includes(app.spec.source.repoURL),
            sync: pref.syncFilter.length === 0 || pref.syncFilter.includes(app.status.sync.status),
            health: pref.healthFilter.length === 0 || pref.healthFilter.includes(app.status.health.status),
            namespaces: pref.namespacesFilter.length === 0 || pref.namespacesFilter.some(ns => app.spec.destination.namespace && minimatch(app.spec.destination.namespace, ns)),
            clusters: pref.clustersFilter.length === 0 || pref.clustersFilter.some(server => server === (app.spec.destination.server || app.spec.destination.name)),
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
        options={getOptions(props.apps, 'sync', app => app.status.sync.status, Object.keys(SyncStatuses), s => (
            <ComparisonStatusIcon status={s as SyncStatusCode} noSpin={true} />
        ))}
    />
);

const HealthFilter = (props: AppFilterProps) => (
    <Filter
        label='HEALTH STATUS'
        selected={props.pref.healthFilter}
        setSelected={s => props.onChange({...props.pref, healthFilter: s})}
        options={getOptions(props.apps, 'health', app => app.status.health.status, Object.keys(HealthStatuses), s => (
            <HealthStatusIcon state={{status: s as HealthStatusCode, message: ''}} noSpin={true} />
        ))}
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
    const [projects, loading, error] = useData(() => services.projects.list('items.metadata.name'), null, () => null);
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

export const ApplicationsFilter = (props: AppFilterProps) => {
    const [hidden, setHidden] = React.useState(false);

    React.useEffect(() => {
        const handleResize = () => {
            if (window.innerWidth >= 1440) {
                setHidden(false);
            }
        };

        window.addEventListener('resize', debounce(handleResize, 1000));
        return () => window.removeEventListener('resize', handleResize);
    });
    return (
        <React.Fragment>
            <div className='applications-list__filters__title'>
                FILTERS <i className='fa fa-filter' />
                <ActionButton
                    label={hidden ? 'SHOW' : 'HIDE'}
                    action={() => setHidden(!hidden)}
                    style={{marginLeft: 'auto', fontSize: '12px', lineHeight: '5px', display: hidden && 'block'}}
                />
            </div>
            <div className='applications-list__filters'>
                {!hidden && (
                    <React.Fragment>
                        <SyncFilter {...props} />
                        <HealthFilter {...props} />
                        <div className='applications-list__filters__text-filters'>
                            <LabelsFilter {...props} />
                            <ProjectFilter {...props} />
                            <ClusterFilter {...props} />
                            <NamespaceFilter {...props} />
                        </div>
                    </React.Fragment>
                )}
            </div>
        </React.Fragment>
    );
};
