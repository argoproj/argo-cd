import * as React from 'react';
import {ApplicationTree, HealthStatusCode, HealthStatuses, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {AppDetailsPreferences} from '../../../shared/services';
import {Filter, FiltersGroup} from '../filter/filter';
import {ComparisonStatusIcon, HealthStatusIcon} from '../utils';
import {COLORS} from '../../../shared/components';
import {resources} from '../resources';
import * as models from '../../../shared/models';

const uniq = (value: string, index: number, self: string[]) => self.indexOf(value) === index;

// Ownership describes how a resource relates to the application, which is orthogonal to its sync status.
// Orphaned resources are not part of `status.resources` and therefore have no sync status at all.
export const OWNERSHIP_MANAGED = 'Managed';
export const OWNERSHIP_ORPHANED = 'Orphaned';

const OwnershipIcon = ({ownership}: {ownership: string}) =>
    ownership === OWNERSHIP_ORPHANED ? (
        <i title='Not managed by any application' className='fa fa-child' style={{color: COLORS.sync.unknown}} />
    ) : (
        <i title='Managed by this application' className='fa fa-sitemap' style={{color: COLORS.sync.synced}} />
    );

function toOption(label: string) {
    return {label};
}

export interface FiltersProps {
    children?: React.ReactNode;
    pref: AppDetailsPreferences;
    tree: ApplicationTree;
    resourceNodes: models.ResourceStatus[];
    onSetFilter: (items: string[]) => void;
    onClearFilter: () => void;
    collapsed?: boolean;
}

export const Filters = (props: FiltersProps) => {
    const {pref, tree, onSetFilter} = props;

    const onClearFilter = () => {
        setLoading(true);
        props.onClearFilter();
    };

    const resourceFilter = pref.resourceFilter || [];
    const removePrefix = (prefix: string) => (v: string) => v.replace(prefix + ':', '');

    const [loading, setLoading] = React.useState(true);

    const groupedFilters = React.useMemo(() => {
        const update: {[key: string]: string} = {};
        (resourceFilter || []).forEach(pair => {
            const tmp = pair.split(':');
            if (tmp.length === 2) {
                const prefix = tmp[0];
                const cur = update[prefix];
                update[prefix] = `${cur ? cur + ',' : ''}${pair}`;
            }
        });
        return update;
    }, [resourceFilter]);

    React.useEffect(() => {
        if (loading) {
            setLoading(false);
        }
    }, [resourceFilter, loading]);

    const setFilters = (prefix: string, values: string[]) => {
        const groups = {...groupedFilters};
        groups[prefix] = values.map(v => `${prefix}:${v}`).join(',');
        let strings: string[] = [];
        Object.keys(groups).forEach(g => {
            strings = strings.concat(groups[g].split(',').filter(f => f !== ''));
        });
        onSetFilter(strings);
    };

    const ResourceFilter = (p: {label: string; prefix: string; options: {label: string}[]; abbreviations?: Map<string, string>; field?: boolean; radio?: boolean}) => {
        return loading ? (
            <div>Loading...</div>
        ) : (
            <Filter
                label={p.label}
                selected={selectedFor(p.prefix)}
                setSelected={v => setFilters(p.prefix, v)}
                options={p.options}
                abbreviations={p.abbreviations}
                field={!!p.field}
                radio={!!p.radio}
            />
        );
    };

    // we need to include ones that might have been filter in other apps that do not apply to the current app,
    // otherwise the user will not be able to clear them from this panel
    const alreadyFilteredOn = (prefix: string) => resourceFilter.filter(f => f.startsWith(prefix + ':')).map(removePrefix(prefix));

    const selectedFor = (prefix: string) => {
        return groupedFilters[prefix] ? groupedFilters[prefix].split(',').map(removePrefix(prefix)) : [];
    };

    const kinds = tree.nodes
        .map(x => x.kind)
        .concat(alreadyFilteredOn('kind'))
        .filter(uniq)
        .sort();

    const names = tree.nodes
        .map(x => x.name)
        .concat(alreadyFilteredOn('name'))
        .filter(uniq)
        .sort();

    const namespaces = tree.nodes
        .map(x => x.namespace)
        .filter(x => !!x)
        .concat(alreadyFilteredOn('namespace'))
        .filter(uniq)
        .sort();

    // Ownership is only meaningful when the project reports orphaned resources for this application.
    // Keep it visible while a stale ownership filter is applied so the user can still clear it.
    const showOwnershipFilter = (tree.orphanedNodes || []).length > 0 || alreadyFilteredOn('ownership').length > 0;

    // Orphaned resources are absent from `status.resources`, so they have no sync status. They do carry a health
    // status, so only the sync group is hidden when the view is narrowed down to orphaned resources only. It stays
    // visible while a sync filter is applied so it can still be cleared.
    const selectedOwnership = selectedFor('ownership');
    const orphanedOnly = selectedOwnership.length === 1 && selectedOwnership[0] === OWNERSHIP_ORPHANED;
    const showSyncFilter = !orphanedOnly || alreadyFilteredOn('sync').length > 0;

    const getOptionCount = (label: string, filterType: string): number => {
        switch (filterType) {
            case 'Sync':
                return props.resourceNodes.filter(res => res.status === SyncStatuses[label]).length;
            case 'Health':
                return props.resourceNodes.filter(res => res.health?.status === HealthStatuses[label]).length;
            case 'Kind':
                return props.resourceNodes.reduce((count, res) => (res.group && label === 'Pod' ? res.group.length : res.kind === label ? count + 1 : count), 0);
            case 'Ownership':
                // Orphaned nodes are only present in resourceNodes once the filter is selected, so count them from the tree.
                return label === OWNERSHIP_ORPHANED ? (tree.orphanedNodes || []).length : props.resourceNodes.filter(res => !res.orphaned).length;
            default:
                return 0;
        }
    };

    return (
        <FiltersGroup title='Resource filters' content={props.children} appliedFilter={pref.resourceFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            {ResourceFilter({label: 'NAME', prefix: 'name', options: names.map(toOption), field: true})}
            {ResourceFilter({
                label: 'KINDS',
                prefix: 'kind',
                options: kinds.map(label => ({
                    label,
                    count: getOptionCount(label, 'Kind')
                })),
                abbreviations: resources,
                field: true
            })}
            {showSyncFilter &&
                ResourceFilter({
                    label: 'SYNC STATUS',
                    prefix: 'sync',
                    options: ['Synced', 'OutOfSync'].map(label => ({
                        label,
                        count: getOptionCount(label, 'Sync'),
                        icon: <ComparisonStatusIcon status={label as SyncStatusCode} noSpin={true} />
                    }))
                })}
            {ResourceFilter({
                label: 'HEALTH STATUS',
                prefix: 'health',
                options: ['Progressing', 'Suspended', 'Healthy', 'Degraded', 'Missing', 'Unknown'].map(label => ({
                    label,
                    count: getOptionCount(label, 'Health'),
                    icon: <HealthStatusIcon state={{status: label as HealthStatusCode, message: ''}} noSpin={true} />
                }))
            })}
            {showOwnershipFilter &&
                ResourceFilter({
                    label: 'OWNERSHIP',
                    prefix: 'ownership',
                    options: [OWNERSHIP_MANAGED, OWNERSHIP_ORPHANED].map(label => ({
                        label,
                        count: getOptionCount(label, 'Ownership'),
                        icon: <OwnershipIcon ownership={label} />
                    }))
                })}
            {namespaces.length > 1 && ResourceFilter({label: 'NAMESPACES', prefix: 'namespace', options: (namespaces || []).filter(l => l && l !== '').map(toOption), field: true})}
        </FiltersGroup>
    );
};
