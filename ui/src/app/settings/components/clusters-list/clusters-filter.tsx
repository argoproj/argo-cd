import * as React from 'react';
import * as models from '../../../shared/models';
import {COLORS} from '../../../shared/components/colors';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';
import {getFilterCounts, getFilterOptions} from '../list-filter-utils';

export interface ClustersListPreferences {
    statusFilter: string[];
    credentialFilter: string[];
}

export interface FilterResult {
    status: boolean;
    credential: boolean;
}

export interface FilteredCluster extends models.Cluster {
    filterResult: FilterResult;
}

export class ClustersListPreferencesHelper {
    public static clearFilters(pref: ClustersListPreferences) {
        pref.statusFilter = [];
        pref.credentialFilter = [];
    }
}

export function getCredentialType(cluster: models.Cluster): string {
    if (cluster.config?.awsAuthConfig) {
        return 'IAM Auth';
    }
    if (cluster.config?.execProviderConfig) {
        return 'External Provider';
    }
    return 'Token/Basic Auth';
}

export function getClusterFilterResults(clusters: models.Cluster[], pref: ClustersListPreferences): FilteredCluster[] {
    return clusters.map(cluster => ({
        ...cluster,
        filterResult: {
            status: pref.statusFilter.length === 0 || pref.statusFilter.includes(cluster.info.connectionState.status),
            credential: pref.credentialFilter.length === 0 || pref.credentialFilter.includes(getCredentialType(cluster))
        }
    }));
}

export function filterClusters(clusters: FilteredCluster[]): models.Cluster[] {
    return clusters.filter(cluster => Object.values(cluster.filterResult).every(v => v));
}

interface ClustersFilterProps {
    clusters: FilteredCluster[];
    pref: ClustersListPreferences;
    onChange: (newPref: ClustersListPreferences) => void;
    collapsed?: boolean;
}

const getStatusIcon = (status: string) => {
    switch (status) {
        case models.ConnectionStatuses.Successful:
            return <i className='fa fa-check-circle' style={{color: COLORS.connection_status.successful}} />;
        case models.ConnectionStatuses.Failed:
            return <i className='fa fa-times-circle' style={{color: COLORS.operation.failed}} />;
        case models.ConnectionStatuses.Unknown:
            return <i className='fa fa-exclamation-circle' style={{color: COLORS.connection_status.unknown}} />;
        default:
            return null;
    }
};

const StatusFilter = (props: ClustersFilterProps) => (
    <Filter
        label='CONNECTION STATUS'
        selected={props.pref.statusFilter}
        setSelected={s => props.onChange({...props.pref, statusFilter: s})}
        options={getFilterOptions(
            props.clusters,
            'status',
            cluster => cluster.info.connectionState.status,
            [models.ConnectionStatuses.Successful, models.ConnectionStatuses.Failed, models.ConnectionStatuses.Unknown],
            getStatusIcon
        )}
    />
);

const CredentialFilter = React.memo((props: ClustersFilterProps) => {
    const credentialOptions = React.useMemo(() => {
        const credTypes = Array.from(new Set(props.clusters.map(cluster => getCredentialType(cluster))));
        const counts = getFilterCounts(props.clusters, 'credential', getCredentialType, credTypes);
        return credTypes.map(type => ({
            label: type,
            count: counts.get(type)
        }));
    }, [props.clusters]);

    return (
        <Filter
            label='CREDENTIAL TYPE'
            selected={props.pref.credentialFilter}
            setSelected={s => props.onChange({...props.pref, credentialFilter: s})}
            options={credentialOptions}
        />
    );
});

export const ClustersFilter = (props: ClustersFilterProps) => {
    const appliedFilter = [...(props.pref.statusFilter || []), ...(props.pref.credentialFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        ClustersListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Cluster filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <CredentialFilter {...props} />
            <StatusFilter {...props} />
        </FiltersGroup>
    );
};
