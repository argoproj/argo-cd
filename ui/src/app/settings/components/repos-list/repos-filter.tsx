import * as React from 'react';
import * as models from '../../../shared/models';
import {COLORS} from '../../../shared/components/colors';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';

export interface ReposListPreferences {
    typeFilter: string[];
    projectFilter: string[];
    statusFilter: string[];
}

export interface FilterResult {
    type: boolean;
    project: boolean;
    status: boolean;
}

export interface FilteredRepo extends models.Repository {
    filterResult: FilterResult;
}

export class ReposListPreferencesHelper {
    public static clearFilters(pref: ReposListPreferences) {
        pref.typeFilter = [];
        pref.projectFilter = [];
        pref.statusFilter = [];
    }
}

export function getRepoFilterResults(repos: models.Repository[], pref: ReposListPreferences): FilteredRepo[] {
    return repos.map(repo => ({
        ...repo,
        filterResult: {
            type: pref.typeFilter.length === 0 || pref.typeFilter.includes(repo.type || 'git'),
            project: pref.projectFilter.length === 0 || (repo.project && pref.projectFilter.includes(repo.project)),
            status: pref.statusFilter.length === 0 || pref.statusFilter.includes(repo.connectionState.status)
        }
    }));
}

export function filterRepos(repos: FilteredRepo[]): models.Repository[] {
    return repos.filter(repo => Object.values(repo.filterResult).every(v => v));
}

const getCounts = (repos: FilteredRepo[], filterType: keyof FilterResult, filter: (repo: models.Repository) => string, init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    repos
        .filter(repo => filter(repo) && Object.keys(repo.filterResult).every((key: keyof FilterResult) => key === filterType || repo.filterResult[key]))
        .forEach(repo => map.set(filter(repo), (map.get(filter(repo)) || 0) + 1));
    return map;
};

const getOptions = (
    repos: FilteredRepo[],
    filterType: keyof FilterResult,
    filter: (repo: models.Repository) => string,
    keys: string[],
    getIcon?: (k: string) => React.ReactNode
) => {
    const counts = getCounts(repos, filterType, filter, keys);
    return keys.map(k => ({
        label: k,
        icon: getIcon && getIcon(k),
        count: counts.get(k)
    }));
};

const optionsFrom = (options: string[], filter: string[]) => {
    return options.filter(s => filter.indexOf(s) === -1).map(item => ({label: item}));
};

interface ReposFilterProps {
    repos: FilteredRepo[];
    pref: ReposListPreferences;
    onChange: (newPref: ReposListPreferences) => void;
    collapsed?: boolean;
}

const getTypeIcon = (type: string) => <i className={'icon argo-icon-' + type} style={{fontSize: '16px', display: 'inline-block', verticalAlign: 'middle'}} />;

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
        selected={props.pref.typeFilter}
        setSelected={s => props.onChange({...props.pref, typeFilter: s})}
        options={getOptions(props.repos, 'type', repo => repo.type || 'git', ['git', 'helm'], getTypeIcon)}
    />
);

const ProjectFilter = React.memo((props: ReposFilterProps) => {
    const projectOptions = React.useMemo(
        () => optionsFrom(Array.from(new Set(props.repos.map(repo => repo.project).filter((item): item is string => !!item && item.trim() !== ''))), props.pref.projectFilter),
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
        options={getOptions(
            props.repos,
            'status',
            repo => repo.connectionState.status,
            [models.ConnectionStatuses.Successful, models.ConnectionStatuses.Failed, models.ConnectionStatuses.Unknown],
            getStatusIcon
        )}
    />
);

export const ReposFilter = (props: ReposFilterProps) => {
    const appliedFilter = [...(props.pref.typeFilter || []), ...(props.pref.projectFilter || []), ...(props.pref.statusFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        ReposListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Repository filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <ProjectFilter {...props} />
            <TypeFilter {...props} />
            <StatusFilter {...props} />
        </FiltersGroup>
    );
};
