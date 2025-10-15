import {useData} from 'argo-ui/v2';
import * as minimatch from 'minimatch';
import * as React from 'react';
import {Cluster, HealthStatusCode, HealthStatuses, Resource, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {ResourcesListPreferences, services} from '../../../shared/services';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';
import {ComparisonStatusIcon, HealthStatusIcon} from '../../../applications/components/utils';

export interface FilterResult {
    sync: boolean;
    health: boolean;
    namespaces: boolean;
    clusters: boolean;
}

export interface FilteredResource extends Resource {
    filterResult: FilterResult;
}

export function getFilterResults(resources: Resource[], pref: ResourcesListPreferences): FilteredResource[] {
    return resources.map(app => ({
        ...app,
        filterResult: {
            sync: pref.syncFilter.length === 0 || pref.syncFilter.includes(app.status),
            health: pref.healthFilter.length === 0 || pref.healthFilter.includes(app.health?.status),
            namespaces: pref.namespacesFilter.length === 0 || pref.namespacesFilter.some(ns => app.namespace && minimatch(app.namespace, ns)),
            clusters:
                pref.clustersFilter.length === 0 ||
                pref.clustersFilter.some(filterString => {
                    const match = filterString.match('^(.*) [(](http.*)[)]$');
                    if (match?.length === 3) {
                        const [, name, url] = match;
                        return url === app.clusterServer || name === app.clusterName;
                    } else {
                        const inputMatch = filterString.match('^http.*$');
                        return (inputMatch && inputMatch[0] === app.clusterServer) || (app.clusterName && minimatch(app.clusterName, filterString));
                    }
                }),
            apiGroup: pref.apiGroupFilter.length === 0 || pref.apiGroupFilter.includes(app.group),
            kind: pref.kindFilter.length === 0 || pref.kindFilter.includes(app.kind)
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
    apps: FilteredResource[];
    pref: ResourcesListPreferences;
    onChange: (newPrefs: ResourcesListPreferences) => void;
    children?: React.ReactNode;
    collapsed?: boolean;
}

const getCounts = (apps: FilteredResource[], filterType: keyof FilterResult, filter: (app: Resource) => string, init?: string[]) => {
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

const getOptions = (apps: FilteredResource[], filterType: keyof FilterResult, filter: (app: Resource) => string, keys: string[], getIcon?: (k: string) => React.ReactNode) => {
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
            app => app.status,
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
            app => app.health?.status || HealthStatuses.Unknown,
            Object.keys(HealthStatuses),
            s => (
                <HealthStatusIcon state={{status: s as HealthStatusCode, message: ''}} noSpin={true} />
            )
        )}
    />
);

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
    const getClusterDetail = (dest: Resource, clusterList: Cluster[]): string => {
        const cluster = (clusterList || []).find(target => target.name === dest.clusterName || target.server === dest.clusterServer);
        if (!cluster) {
            return dest.clusterServer || dest.clusterName;
        }
        if (cluster.name === cluster.server) {
            return cluster.name;
        }
        return `${cluster.name} (${cluster.server})`;
    };

    const [clusters, loading, error] = useData(() => services.clusters.list());
    const clusterOptions = optionsFrom(Array.from(new Set(props.apps.map(app => getClusterDetail(app, clusters)).filter(item => !!item))), props.pref.clustersFilter);

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
    const namespaceOptions = optionsFrom(Array.from(new Set(props.apps.map(app => app.namespace).filter(item => !!item))), props.pref.namespacesFilter);
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

const ApiGroupFilter = (props: AppFilterProps) => {
    const apiGroupOptions = optionsFrom(Array.from(new Set(props.apps.map(app => app.group).filter(item => !!item))), props.pref.apiGroupFilter);
    return (
        <Filter
            label='API GROUPS'
            selected={props.pref.apiGroupFilter}
            setSelected={s => props.onChange({...props.pref, apiGroupFilter: s})}
            field={true}
            options={apiGroupOptions}
        />
    );
};

const KindFilter = (props: AppFilterProps) => {
    const kindOptions = optionsFrom(Array.from(new Set(props.apps.map(app => app.kind).filter(item => !!item))), props.pref.kindFilter);
    return <Filter label='KINDS' selected={props.pref.kindFilter} setSelected={s => props.onChange({...props.pref, kindFilter: s})} field={true} options={kindOptions} />;
};

export const ResourcesFilter = (props: AppFilterProps) => {
    return (
        <FiltersGroup title='Resources filters' content={props.children} collapsed={props.collapsed}>
            <SyncFilter {...props} />
            <HealthFilter {...props} />
            <ProjectFilter {...props} />
            <ClusterFilter {...props} />
            <NamespaceFilter {...props} />
            <ApiGroupFilter {...props} />
            <KindFilter {...props} />
        </FiltersGroup>
    );
};
