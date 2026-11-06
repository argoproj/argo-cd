import * as React from 'react';
import * as models from '../../../shared/models';
import {COLORS} from '../../../shared/components/colors';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';
import {capitalize, getFilterOptions} from '../list-filter-utils';

export interface ReposListPreferences {
    typeFilter: string[];
    projectFilter: string[];
    statusFilter: string[];
    credentialTypeFilter: string[]; // read or write
    templateFilter: string[]; // template or repository
}

export interface FilterResult {
    type: boolean;
    project: boolean;
    status: boolean;
    credentialType: boolean;
    template: boolean;
}

// Unified type for both repositories and credential templates
// Uses discriminated union - only one of these four will be defined
export interface UnifiedRepo {
    readRepo?: models.Repository;
    writeRepo?: models.Repository;
    readCred?: models.RepoCreds;
    writeCred?: models.RepoCreds;
}

export interface FilteredRepo extends UnifiedRepo {
    filterResult: FilterResult;
}

// Helper functions to access properties
export function getRepoUrl(item: UnifiedRepo): string {
    return item.readRepo?.repo || item.writeRepo?.repo || item.readCred?.url || item.writeCred?.url || '';
}

export function getRepoName(item: UnifiedRepo): string {
    // Only repositories have a name; templates are left blank when no name is defined
    return item.readRepo?.name || item.writeRepo?.name || '';
}

export function getRepoType(item: UnifiedRepo): string {
    // Check if OCI is enabled (for Helm OCI registries)
    if (item.readRepo?.enableOCI || item.writeRepo?.enableOCI || item.readCred?.enableOCI || item.writeCred?.enableOCI) {
        return 'oci';
    }
    return item.readRepo?.type || item.writeRepo?.type || item.readCred?.type || item.writeCred?.type || 'git';
}

export function getRepoProject(item: UnifiedRepo): string | undefined {
    return item.readRepo?.project || item.writeRepo?.project;
}

export function getConnectionState(item: UnifiedRepo): models.ConnectionState | undefined {
    return item.readRepo?.connectionState || item.writeRepo?.connectionState;
}

export function isWrite(item: UnifiedRepo): boolean {
    return !!(item.writeRepo || item.writeCred);
}

export function isTemplate(item: UnifiedRepo): boolean {
    return !!(item.readCred || item.writeCred);
}

export class ReposListPreferencesHelper {
    public static clearFilters(pref: ReposListPreferences) {
        pref.typeFilter = [];
        pref.projectFilter = [];
        pref.statusFilter = [];
        pref.credentialTypeFilter = [];
        pref.templateFilter = [];
    }
}

export function getRepoFilterResults(repos: UnifiedRepo[], pref: ReposListPreferences): FilteredRepo[] {
    return repos.map(repo => ({
        ...repo,
        filterResult: {
            type: pref.typeFilter.length === 0 || pref.typeFilter.includes(getRepoType(repo)),
            project: pref.projectFilter.length === 0 || (getRepoProject(repo) && pref.projectFilter.includes(getRepoProject(repo)!)),
            status: pref.statusFilter.length === 0 || (getConnectionState(repo) && pref.statusFilter.includes(getConnectionState(repo)!.status)),
            credentialType: pref.credentialTypeFilter.length === 0 || pref.credentialTypeFilter.includes(isWrite(repo) ? 'write' : 'read'),
            template: pref.templateFilter.length === 0 || pref.templateFilter.includes(isTemplate(repo) ? 'template' : 'repository')
        }
    }));
}

export function filterRepos(repos: FilteredRepo[]): UnifiedRepo[] {
    return repos.filter(repo => Object.values(repo.filterResult).every(v => v));
}

const optionsFrom = (options: string[], filter: string[]) => {
    return options.filter(s => filter.indexOf(s) === -1).map(item => ({label: item}));
};

interface ReposFilterProps {
    repos: FilteredRepo[];
    pref: ReposListPreferences;
    onChange: (newPref: ReposListPreferences) => void;
    collapsed?: boolean;
}

const getTypeIcon = (type: string) => <i className={'icon argo-icon-' + type} style={{fontSize: '16px', display: 'flex', alignItems: 'center'}} />;

const getTypeLabel = (type: string) => {
    switch (type) {
        case 'oci':
            return 'OCI';
        default:
            return type.charAt(0).toUpperCase() + type.slice(1);
    }
};

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

const TypeFilter = (props: ReposFilterProps) => (
    <Filter
        label='TYPE'
        selected={props.pref.typeFilter.map(getTypeLabel)}
        setSelected={s => props.onChange({...props.pref, typeFilter: s.map(v => v.toLowerCase())})}
        options={getFilterOptions(props.repos, 'type', getRepoType, ['git', 'helm', 'oci'], getTypeIcon, getTypeLabel)}
    />
);

const ProjectFilter = React.memo((props: ReposFilterProps) => {
    const projectOptions = React.useMemo(
        () => optionsFrom(Array.from(new Set(props.repos.map(getRepoProject).filter((item): item is string => !!item && item.trim() !== ''))), props.pref.projectFilter),
        [props.repos, props.pref.projectFilter]
    );
    return (
        <Filter label='PROJECT' selected={props.pref.projectFilter} setSelected={s => props.onChange({...props.pref, projectFilter: s})} field={true} options={projectOptions} />
    );
});

const StatusFilter = (props: ReposFilterProps) => (
    <Filter
        label='CONNECTION STATUS'
        selected={props.pref.statusFilter}
        setSelected={s => props.onChange({...props.pref, statusFilter: s})}
        options={getFilterOptions(
            props.repos,
            'status',
            repo => getConnectionState(repo)?.status,
            [models.ConnectionStatuses.Successful, models.ConnectionStatuses.Failed, models.ConnectionStatuses.Unknown],
            getStatusIcon,
            capitalize
        )}
    />
);

const PermissionFilter = (props: ReposFilterProps) => (
    <Filter
        label='PERMISSION'
        selected={props.pref.credentialTypeFilter.map(s => s.charAt(0).toUpperCase() + s.slice(1))}
        setSelected={s => props.onChange({...props.pref, credentialTypeFilter: s.map(v => v.toLowerCase())})}
        options={getFilterOptions(props.repos, 'credentialType', repo => (isWrite(repo) ? 'write' : 'read'), ['read', 'write'], undefined, capitalize)}
        radio={true}
    />
);

const CategoryFilter = (props: ReposFilterProps) => (
    <Filter
        label='CATEGORY'
        selected={props.pref.templateFilter.map(s => s.charAt(0).toUpperCase() + s.slice(1))}
        setSelected={s => props.onChange({...props.pref, templateFilter: s.map(v => v.toLowerCase())})}
        options={getFilterOptions(props.repos, 'template', repo => (isTemplate(repo) ? 'template' : 'repository'), ['repository', 'template'], undefined, capitalize)}
    />
);

export const ReposFilter = (props: ReposFilterProps) => {
    const appliedFilter = [
        ...(props.pref.typeFilter || []),
        ...(props.pref.projectFilter || []),
        ...(props.pref.statusFilter || []),
        ...(props.pref.credentialTypeFilter || []),
        ...(props.pref.templateFilter || [])
    ];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        ReposListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Repository filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <ProjectFilter {...props} />
            <CategoryFilter {...props} />
            <TypeFilter {...props} />
            <PermissionFilter {...props} />
            <StatusFilter {...props} />
        </FiltersGroup>
    );
};
