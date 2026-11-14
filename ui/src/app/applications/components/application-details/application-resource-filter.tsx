import * as React from 'react';
import {Checkbox} from 'argo-ui/v2';
import {ApplicationTree, HealthStatusCode, HealthStatuses, SyncStatusCode, SyncStatuses} from '../../../shared/models';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {Context} from '../../../shared/context';
import {Filter, FiltersGroup} from '../filter/filter';
import {ComparisonStatusIcon, HealthStatusIcon} from '../utils';
import {resources} from '../resources';
import * as models from '../../../shared/models';

const uniq = (value: string, index: number, self: string[]) => self.indexOf(value) === index;

function toOption(label: string) {
    return {label};
}

export const POD_STATUS_REASON_INFO = 'Status Reason';

export interface FilterInput {
    name: string[];
    kind: string[];
    health: string[];
    sync: string[];
    namespace: string[];
    podStatus: string[];
}

// splitResourceFilter splits a filter item on its first ':' only, so values such as pod status reasons
// (e.g. 'podStatus:Init:CrashLoopBackOff') keep their own ':' characters.
export function splitResourceFilter(item: string): [string, string] | null {
    const index = item.indexOf(':');
    if (index <= 0) {
        return null;
    }
    return [item.substring(0, index), item.substring(index + 1)];
}

export function parseResourceFilter(filters: string[]): FilterInput {
    const result: FilterInput = {name: [], kind: [], health: [], sync: [], namespace: [], podStatus: []};
    for (const item of filters || []) {
        const parsed = splitResourceFilter(item);
        if (parsed && Object.prototype.hasOwnProperty.call(result, parsed[0])) {
            result[parsed[0] as keyof FilterInput].push(parsed[1]);
        }
    }
    return result;
}

function escapeRegex(input: string): string {
    return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function nameMatchesWildcardFilters(name: string, patterns: string[]): boolean {
    const regularExpression = new RegExp(
        patterns
            // Escape any regex input to ensure only * can be used
            .map(pattern => '^' + escapeRegex(pattern) + '$')
            // Replace any escaped * with proper regex
            .map(pattern => pattern.replace(/\\\*/g, '.*'))
            // Join all filterInputs to a single regular expression
            .join('|'),
        'gi'
    );
    return regularExpression.test(name);
}

// Resource filter categories that don't apply to the ApplicationSet view. The AppSet view hides these
// filter controls, and its generated Application nodes don't carry kind/sync/health status.
const APPLICATION_SET_HIDDEN_FILTERS = ['kind:', 'sync:', 'health:'];

// Resource filter categories that don't apply to the pods view, which only shows pods.
const POD_VIEW_HIDDEN_FILTERS = ['kind:'];

// Resource filter categories that only apply to the pods view.
const POD_VIEW_ONLY_FILTERS = ['podStatus:'];

// getEffectiveResourceFilter returns the resource filters that should actually be applied for the
// current view. The resource filter is a single shared preference across the Application and
// ApplicationSet views, so a filter set in an Application view (e.g. sync:OutOfSync) would otherwise
// linger in the AppSet view and silently hide all generated Applications with no visible control to
// clear it (those controls are hidden for the AppSet view). To keep the effective filter consistent
// with the controls actually shown, we strip the hidden filter categories for the ApplicationSet view.
// The same applies to the pods view: kind filters are hidden there, and pod status filters are only
// shown there.
export function getEffectiveResourceFilter(isApplicationSet: boolean, resourceFilter?: string[], view?: string): string[] {
    const hidden = view === 'pods' ? [...POD_VIEW_HIDDEN_FILTERS] : [...POD_VIEW_ONLY_FILTERS];
    if (isApplicationSet) {
        hidden.push(...APPLICATION_SET_HIDDEN_FILTERS);
    }
    return (resourceFilter || []).filter(f => !hidden.some(prefix => f.startsWith(prefix)));
}

export function getPodStatusReason(pod: {info?: models.InfoItem[]}): string {
    return (pod.info || []).find(i => i.name === POD_STATUS_REASON_INFO)?.value || '';
}

// getRootResource walks the first parent reference of the node up to its top-level resource.
export function getRootResource<T extends models.ResourceNode>(node: T, nodesByUid: Map<string, T>): T {
    let cur = node;
    const visited = new Set<string>();
    while (cur && (cur.parentRefs || []).length > 0 && !visited.has(cur.uid)) {
        visited.add(cur.uid);
        const parent = nodesByUid.get(cur.parentRefs[0].uid);
        if (!parent) {
            break;
        }
        cur = parent;
    }
    return cur;
}

// uniqueRootResources returns the distinct top-level resources of the given pods. Pods are not synced directly, so the
// pods view counts sync statuses on these resources, like the other views count the resources that have a sync status.
export function uniqueRootResources<T extends models.ResourceNode>(pods: T[], getRoot: (pod: T) => T | undefined): T[] {
    const roots = new Map<string, T>();
    for (const pod of pods) {
        const root = getRoot(pod);
        if (root) {
            roots.set(root.uid || `${root.group}/${root.kind}/${root.namespace}/${root.name}`, root);
        }
    }
    return Array.from(roots.values());
}

// podMatchesFilter checks a pod against the filter. Name, namespace, health and pod status are matched against the
// pod itself, while sync status is matched against the pod's top-level resource since pods are not synced directly.
export function podMatchesFilter(pod: models.ResourceNode & {health?: models.HealthStatus}, root: Partial<models.ResourceStatus> | undefined, filter: FilterInput): boolean {
    const syncStatuses = filter.sync.reduce((acc, item) => acc.concat(item === 'OutOfSync' ? ['OutOfSync', 'Unknown'] : [item]), [] as string[]);
    return (
        (filter.name.length === 0 || nameMatchesWildcardFilters(pod.name, filter.name)) &&
        (filter.namespace.length === 0 || filter.namespace.includes(pod.namespace)) &&
        (filter.health.length === 0 || (!!pod.health && filter.health.includes(pod.health.status))) &&
        (syncStatuses.length === 0 || !!root?.hook || (!!root?.status && syncStatuses.includes(root.status))) &&
        (filter.podStatus.length === 0 || filter.podStatus.includes(getPodStatusReason(pod)))
    );
}

export interface FiltersProps {
    children?: React.ReactNode;
    pref: AppDetailsPreferences;
    tree: ApplicationTree;
    resourceNodes: models.ResourceStatus[];
    // syncStatusNodes are the resources counted for the sync status options. Defaults to resourceNodes.
    syncStatusNodes?: models.ResourceStatus[];
    onSetFilter: (items: string[]) => void;
    onClearFilter: () => void;
    collapsed?: boolean;
    isApplicationSet?: boolean;
    // view is the current application details view. In the pods view, the tree nodes are expected to be pods only.
    view?: string;
}

export const Filters = (props: FiltersProps) => {
    const ctx = React.useContext(Context);

    const {pref, tree, onSetFilter, isApplicationSet, view} = props;
    const isPodView = view === 'pods';

    const onClearFilter = () => {
        setLoading(true);
        props.onClearFilter();
    };

    const resourceFilter = pref.resourceFilter || [];
    const removePrefix = (prefix: string) => (v: string) => (v.startsWith(prefix + ':') ? v.substring(prefix.length + 1) : v);

    const [loading, setLoading] = React.useState(true);

    const groupedFilters = React.useMemo(() => {
        const update: {[key: string]: string} = {};
        (resourceFilter || []).forEach(pair => {
            const tmp = splitResourceFilter(pair);
            if (tmp) {
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

    const podStatuses = isPodView
        ? tree.nodes
              .map(getPodStatusReason)
              .filter(x => !!x)
              .concat(alreadyFilteredOn('podStatus'))
              .filter(uniq)
              .sort()
        : [];

    const selectedFor = (prefix: string) => {
        return groupedFilters[prefix] ? groupedFilters[prefix].split(',').map(removePrefix(prefix)) : [];
    };

    const getOptionCount = (label: string, filterType: string): number => {
        switch (filterType) {
            case 'Sync':
                return (props.syncStatusNodes || props.resourceNodes).filter(res => res.status === SyncStatuses[label]).length;
            case 'Health':
                return props.resourceNodes.filter(res => res.health?.status === HealthStatuses[label]).length;
            case 'PodStatus':
                return props.resourceNodes.filter(res => getPodStatusReason(res as {info?: models.InfoItem[]}) === label).length;
            case 'Kind':
                return props.resourceNodes.reduce((count, res) => (res.group && label === 'Pod' ? res.group.length : res.kind === label ? count + 1 : count), 0);
            default:
                return 0;
        }
    };

    return (
        <FiltersGroup
            title='Resource filters'
            content={props.children}
            appliedFilter={getEffectiveResourceFilter(!!isApplicationSet, resourceFilter, view)}
            onClearFilter={onClearFilter}
            collapsed={props.collapsed}>
            {ResourceFilter({label: 'NAME', prefix: 'name', options: names.map(toOption), field: true})}
            {!isApplicationSet &&
                !isPodView &&
                ResourceFilter({
                    label: 'KINDS',
                    prefix: 'kind',
                    options: kinds.map(label => ({
                        label,
                        count: getOptionCount(label, 'Kind')
                    })),
                    abbreviations: resources,
                    field: true
                })}
            {!isApplicationSet &&
                ResourceFilter({
                    label: 'SYNC STATUS',
                    prefix: 'sync',
                    options: ['Synced', 'OutOfSync'].map(label => ({
                        label,
                        count: getOptionCount(label, 'Sync'),
                        icon: <ComparisonStatusIcon status={label as SyncStatusCode} noSpin={true} />
                    }))
                })}
            {!isApplicationSet &&
                ResourceFilter({
                    label: 'HEALTH STATUS',
                    prefix: 'health',
                    options: ['Progressing', 'Suspended', 'Healthy', 'Degraded', 'Missing', 'Unknown'].map(label => ({
                        label,
                        count: getOptionCount(label, 'Health'),
                        icon: <HealthStatusIcon state={{status: label as HealthStatusCode, message: ''}} noSpin={true} />
                    }))
                })}
            {isPodView &&
                podStatuses.length > 0 &&
                ResourceFilter({
                    label: 'POD STATUS',
                    prefix: 'podStatus',
                    options: podStatuses.map(label => ({
                        label,
                        count: getOptionCount(label, 'PodStatus')
                    }))
                })}
            {namespaces.length > 1 && ResourceFilter({label: 'NAMESPACES', prefix: 'namespace', options: (namespaces || []).filter(l => l && l !== '').map(toOption), field: true})}
            {!isPodView && (tree.orphanedNodes || []).length > 0 && (
                <div className={`filter filter__item ${pref.orphanedResources ? 'filter__item--selected' : ''}`}>
                    <Checkbox
                        value={!!pref.orphanedResources}
                        onChange={val => {
                            ctx.navigation.goto('.', {orphaned: val}, {replace: true});
                            services.viewPreferences.updatePreferences({appDetails: {...pref, orphanedResources: val}});
                        }}
                        style={{
                            marginRight: '8px',
                            marginLeft: '8px'
                        }}
                    />
                    <div className='filter__item__label'>Show Orphaned</div>
                </div>
            )}
        </FiltersGroup>
    );
};
