import * as React from 'react';
import {Checkbox} from 'argo-ui';
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

export const Filters = (props: {
    children?: React.ReactNode;
    pref: AppDetailsPreferences;
    tree: ApplicationTree;
    resourceNodes: models.ResourceStatus[];
    onSetFilter: (items: string[]) => void;
    onClearFilter: () => void;
}) => {
    const ctx = React.useContext(Context);

    const {pref, tree, onSetFilter} = props;

    const onClearFilter = () => {
        setLoading(true);
        props.onClearFilter();
    };

    const shown = pref.hideFilters;
    const setShown = (val: boolean) => services.viewPreferences.updatePreferences({appDetails: {...pref, hideFilters: val}});

    const resourceFilter = pref.resourceFilter || [];
    const removePrefix = (prefix: string) => (v: string) => v.replace(prefix + ':', '');

    const [groupedFilters, setGroupedFilters] = React.useState<{[key: string]: string}>({});
    const [loading, setLoading] = React.useState(true);

    React.useEffect(() => {
        const update: {[key: string]: string} = {};
        (resourceFilter || []).forEach(pair => {
            const tmp = pair.split(':');
            if (tmp.length === 2) {
                const prefix = tmp[0];
                const cur = update[prefix];
                update[prefix] = `${cur ? cur + ',' : ''}${pair}`;
            }
        });
        setGroupedFilters(update);
        setLoading(false);
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

    const selectedFor = (prefix: string) => {
        return groupedFilters[prefix] ? groupedFilters[prefix].split(',').map(removePrefix(prefix)) : [];
    };

    const getOptionCount = (label: string, filterType: string): number => {
        switch (filterType) {
            case 'Sync':
                return props.resourceNodes.filter(res => res.status === SyncStatuses[label]).length;
            case 'Health':
                return props.resourceNodes.filter(res => res.health?.status === HealthStatuses[label]).length;
            case 'Kind':
                return props.resourceNodes.filter(res => res.kind === label).length;
            default:
                return 0;
        }
    };

    return (
        <FiltersGroup content={props.children} appliedFilter={pref.resourceFilter} onClearFilter={onClearFilter} setShown={setShown} expanded={shown}>
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
            {ResourceFilter({
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
                options: ['Healthy', 'Progressing', 'Degraded', 'Suspended', 'Missing', 'Unknown'].map(label => ({
                    label,
                    count: getOptionCount(label, 'Health'),
                    icon: <HealthStatusIcon state={{status: label as HealthStatusCode, message: ''}} noSpin={true} />
                }))
            })}
            {namespaces.length > 1 && ResourceFilter({label: 'NAMESPACES', prefix: 'namespace', options: (namespaces || []).filter(l => l && l !== '').map(toOption), field: true})}
            {(tree.orphanedNodes || []).length > 0 && (
                <div className='filter'>
                    <Checkbox
                        checked={!!pref.orphanedResources}
                        id='orphanedFilter'
                        onChange={val => {
                            ctx.navigation.goto('.', {orphaned: val}, {replace: true});
                            services.viewPreferences.updatePreferences({appDetails: {...pref, orphanedResources: val}});
                        }}
                    />{' '}
                    <label htmlFor='orphanedFilter'>SHOW ORPHANED</label>
                </div>
            )}
        </FiltersGroup>
    );
};
