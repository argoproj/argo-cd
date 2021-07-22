import {ActionButton} from 'argo-ui/v2';
import * as React from 'react';
import {ApplicationTree} from '../../../shared/models';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {Filter} from '../filter/filter';
import {useActionOnLargeWindow} from '../utils';

const uniq = (value: string, index: number, self: string[]) => self.indexOf(value) === index;

export const Filters = (props: {pref: AppDetailsPreferences; tree: ApplicationTree; onSetFilter: (items: string[]) => void; onClearFilter: () => void}) => {
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

    useActionOnLargeWindow(() => setShown(true));

    const setFilters = (prefix: string, values: string[]) => {
        const groups = {...groupedFilters};
        groups[prefix] = values.map(v => `${prefix}:${v}`).join(',');
        let strings: string[] = [];
        Object.keys(groups).forEach(g => {
            strings = strings.concat(groups[g].split(',').filter(f => f !== ''));
        });
        onSetFilter(strings);
    };

    const ResourceFilter = (p: {label: string; prefix: string; options: string[]; field?: boolean; radio?: boolean}) => {
        return loading ? (
            <div>Loading...</div>
        ) : (
            <Filter
                label={p.label}
                selected={selectedFor(p.prefix)}
                setSelected={v => setFilters(p.prefix, v)}
                options={p.options.map(label => {
                    return {label};
                })}
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
    const namespaces = tree.nodes
        .map(x => x.namespace)
        .concat(alreadyFilteredOn('namespace'))
        .filter(uniq)
        .sort();

    const selectedFor = (prefix: string) => {
        return groupedFilters[prefix] ? groupedFilters[prefix].split(',').map(removePrefix(prefix)) : [];
    };

    return (
        <>
            <div className='applications-list__filters__title'>
                FILTERS <i className='fa fa-filter' />
                {pref.resourceFilter.length > 0 && (
                    <ActionButton label={'CLEAR ALL'} action={() => onClearFilter()} style={{marginLeft: 'auto', fontSize: '12px', lineHeight: '5px', display: 'block'}} />
                )}
                <ActionButton
                    label={!shown ? 'SHOW' : 'HIDE'}
                    action={() => setShown(!shown)}
                    style={{marginLeft: pref.resourceFilter.length > 0 ? '5px' : 'auto', fontSize: '12px', lineHeight: '5px', display: !shown && 'block'}}
                />
            </div>
            {shown && (
                <div className='applications-list__filters'>
                    {ResourceFilter({label: 'KINDS', prefix: 'kind', options: kinds, field: true})}
                    {ResourceFilter({label: 'SYNC STATUS', prefix: 'sync', options: ['Synced', 'OutOfSync']})}
                    {ResourceFilter({label: 'HEALTH STATUS', prefix: 'health', options: ['Healthy', 'Progressing', 'Degraded', 'Suspended', 'Missing', 'Unknown']})}
                    {namespaces.length > 1 && ResourceFilter({label: 'NAMESPACES', prefix: 'namespace', options: (namespaces || []).filter(l => l && l !== ''), field: true})}
                    {ResourceFilter({label: 'OWNERSHIP', prefix: 'ownership', options: ['Owners', 'Owned']})}
                    {ResourceFilter({label: 'AGE', prefix: 'createdWithin', options: ['1m', '3m', '5m', '15m', '60m'], radio: true})}
                </div>
            )}
        </>
    );
};
