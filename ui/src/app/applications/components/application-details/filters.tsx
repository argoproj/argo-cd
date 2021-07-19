import {HelpIcon} from 'argo-ui';
import {ActionButton, Menu} from 'argo-ui/v2';
import * as React from 'react';
import {ApplicationTree} from '../../../shared/models';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {Filter} from '../filter/filter';

const uniq = (value: string, index: number, self: string[]) => self.indexOf(value) === index;

export const Filters = ({
    pref,
    tree,
    onSetFilter,
    onClearFilter
}: {
    pref: AppDetailsPreferences;
    tree: ApplicationTree;
    onSetFilter: (items: string[]) => void;
    onClearFilter: () => void;
}) => {
    const shown = pref.hideFilters;
    const setShown = (val: boolean) => services.viewPreferences.updatePreferences({appDetails: {...pref, hideFilters: val}});

    const resourceFilter = pref.resourceFilter || [];
    const hasPrefix = (prefix: string) => (v: string) => v.startsWith(prefix + ':');
    const removePrefix = (prefix: string) => (v: string) => v.replace(prefix + ':', '');

    const anyFiltered = pref.resourceFilter.length > 0;

    const isFiltered = (prefix: string, suffix: string) => resourceFilter.includes(`${prefix}:${suffix}`);

    const setFilters = (prefix: string, values: string[], remove?: boolean) => {
        const pairs = values.map(val => `${prefix}:${val}`);
        if (groupedFilters[prefix]) {
            groupedFilters[prefix].concat(pairs);
        } else {
            groupedFilters[prefix] = pairs;
        }
        const strings = Object.keys(groupedFilters).map(g => groupedFilters[g].join(','));
        console.log(pairs);
        console.log(strings);
        onSetFilter(strings);
    };
    const enableFilter = (prefix: string, suffix: string) => {
        const items = resourceFilter.filter(v => !hasPrefix(prefix)(v)).concat([`${prefix}:${suffix}`]);
        onSetFilter(items);
    };

    // this is smarter than it looks at first glance, rather than just un-checked known items,
    // it instead finds out what is enabled, and then removes them, which will be tolerant to weird or unknown items
    const clearFilters = (prefix: string) => {
        return setFilters(prefix, resourceFilter.filter(hasPrefix(prefix)).map(removePrefix(prefix)), true);
    };

    // we need to include ones that might have been filter in other apps that do not apply to the current app,
    // otherwise the user will not be able to clear them from this panel
    const alreadyFilteredOn = (prefix: string) => resourceFilter.filter(hasPrefix(prefix)).map(removePrefix(prefix));

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

    const groupedFilters: {[key: string]: string[]} = {};
    (resourceFilter || []).forEach(pair => {
        const tmp = pair.split(':');
        if (tmp.length == 2) {
            const prefix = tmp[0];
            const suffix = tmp[1];
            if (groupedFilters[prefix]) {
                groupedFilters[prefix].push(suffix);
            } else {
                groupedFilters[prefix] = [suffix];
            }
        }
    });

    return (
        <>
            <div className='applications-list__filters__title'>
                FILTERS <i className='fa fa-filter' />
                {anyFiltered && (
                    <small>
                        <a onClick={() => onClearFilter()}>clear all</a>
                    </small>
                )}
                <ActionButton
                    label={!shown ? 'SHOW' : 'HIDE'}
                    action={() => setShown(!shown)}
                    style={{marginLeft: 'auto', fontSize: '12px', lineHeight: '5px', display: !shown && 'block'}}
                />
            </div>
            {shown && (
                <div className='applications-list__filters'>
                    <Menu items={[{label: 'Parents', action: () => null as any}, {label: 'Children', action: () => null as any}]}>
                        <ActionButton label='Ownership' />
                    </Menu>
                    <Filter
                        label='SYNC STATUS'
                        selected={groupedFilters['sync'] || []}
                        setSelected={vals => {
                            setFilters('sync', vals);
                        }}
                        onClear={onClearFilter}
                        options={[{label: 'Synced'}, {label: 'OutOfSync'}]}
                    />
                    <Filter
                        label='HEALTH STATUS'
                        selected={groupedFilters['health'] || []}
                        setSelected={vals => {
                            setFilters('health', vals);
                        }}
                        onClear={onClearFilter}
                        options={[{label: 'Healthy'}, {label: 'Progressing'}, {label: 'Degraded'}, {label: 'Suspended'}, {label: 'Missing'}, {label: 'Unknown'}]}
                    />
                    <Filter
                        label='KINDS'
                        selected={groupedFilters['kind'] || []}
                        setSelected={vals => {
                            setFilters('kind', vals);
                        }}
                        field={true}
                        onClear={onClearFilter}
                        options={kinds.map(label => {
                            return {label};
                        })}
                    />
                    {namespaces.length > 1 && (
                        <Filter
                            label='NAMESPACES'
                            selected={groupedFilters['namespace'] || []}
                            setSelected={vals => {
                                setFilters('namespace', vals);
                            }}
                            field={true}
                            onClear={onClearFilter}
                            options={(namespaces || [])
                                .filter(l => l && l !== '')
                                .map(label => {
                                    if (label) {
                                        return {label};
                                    }
                                    return {label: ''};
                                })}
                        />
                    )}
                    <div className='filter'>
                        <div className='filter__header'>
                            CREATED WITHIN&nbsp;
                            <HelpIcon title='Use this to find recently created resources, if you want recently synced resources, please raise an issue' />
                            {groupedFilters['createdWithin'] && (
                                <div
                                    className='argo-button argo-button--base argo-button--sm'
                                    style={{marginLeft: 'auto'}}
                                    onClick={() => {
                                        clearFilters('createdWithin');
                                    }}>
                                    <i className='fa fa-times-circle' /> CLEAR
                                </div>
                            )}
                        </div>
                        {[1, 3, 5, 15, 60].map(m => (
                            <div key={m} onClick={() => enableFilter('createdWithin', `${m}m`)} style={{cursor: 'pointer'}}>
                                <input type='radio' checked={isFiltered('createdWithin', `${m}m`)} /> {m}m{' '}
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </>
    );
};
