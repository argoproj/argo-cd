import {ActionButton, debounce, useData} from 'argo-ui/v2';
import * as React from 'react';
import {Application, ApplicationDestination, Cluster, HealthStatusCode, HealthStatuses, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';
import {Filter} from '../filter/filter';
import {ComparisonStatusIcon, HealthStatusIcon} from '../utils';

const optionsFrom = (options: string[], filter: string[]) => {
    return options
        .filter(s => filter.indexOf(s) === -1)
        .map(item => {
            return {label: item};
        });
};

interface AppFilterProps {
    apps: Application[];
    pref: AppsListPreferences;
    onChange: (newPrefs: AppsListPreferences) => void;
}

const getCounts = (apps: Application[], filter: (app: Application) => string, init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    apps.filter(filter).forEach(app => map.set(filter(app), (map.get(filter(app)) || 0) + 1));
    return map;
};

const getOptions = (apps: Application[], filter: (app: Application) => string, keys: string[], getIcon?: (k: string) => React.ReactNode) => {
    const counts = getCounts(apps, filter, keys);
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
        options={getOptions(props.apps, app => app.status.sync.status, Object.keys(SyncStatuses), s => (
            <ComparisonStatusIcon status={s as SyncStatusCode} noSpin={true} />
        ))}
    />
);

const HealthFilter = (props: AppFilterProps) => (
    <Filter
        label='HEALTH STATUS'
        selected={props.pref.healthFilter}
        setSelected={s => props.onChange({...props.pref, healthFilter: s})}
        options={getOptions(props.apps, app => app.status.health.status, Object.keys(HealthStatuses), s => (
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

    const [clusters] = useData(() => services.clusters.list());
    const clusterOptions = optionsFrom(
        Array.from(new Set(props.apps.map(app => getClusterDetail(app.spec.destination, clusters)).filter(item => !!item))),
        props.pref.clustersFilter
    );

    return (
        <Filter label='CLUSTERS' selected={props.pref.clustersFilter} setSelected={s => props.onChange({...props.pref, clustersFilter: s})} field={true} options={clusterOptions} />
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
