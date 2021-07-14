import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {useContext} from 'react';
import {Context} from '../../../shared/context';
import {ApplicationTree} from '../../../shared/models';
import {AppDetailsPreferences} from '../../../shared/services';

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
    const {history, navigation} = useContext(Context);

    const shown = new URLSearchParams(history.location.search).get('showFilters') === 'true';
    const setShown = (showFilters: boolean) => navigation.goto('.', {showFilters});

    const resourceFilter = pref.resourceFilter || [];
    const hasPrefix = (prefix: string) => (v: string) => v.startsWith(prefix + ':');
    const removePrefix = (prefix: string) => (v: string) => v.replace(prefix + ':', '');

    const anyFiltered = pref.resourceFilter.length > 0;
    const isFiltered = (prefix: string, suffix: string) => resourceFilter.includes(`${prefix}:${suffix}`);
    const anyPrefixFiltered = (prefix: string) => resourceFilter.find(hasPrefix(prefix));
    const setFilters = (prefix: string, suffixes: string[], v: boolean) => {
        const filters = suffixes.map(suffix => `${prefix}:${suffix}`);
        const items = resourceFilter.filter(y => !filters.includes(y));
        if (v) {
            items.push(...filters);
        }
        onSetFilter(items);
    };
    const enableFilter = (prefix: string, suffix: string) => {
        const items = resourceFilter.filter(v => !hasPrefix(prefix)(v)).concat([`${prefix}:${suffix}`]);
        onSetFilter(items);
    };
    // this is smarter than it looks at first glance, rather than just un-checked known items,
    // it instead finds out what is enabled, and then removes them, which will be tolerant to weird or unknown items
    const clearFilters = (prefix: string) => {
        return setFilters(prefix, resourceFilter.filter(hasPrefix(prefix)).map(removePrefix(prefix)), false);
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

    const checkbox = (prefix: string, suffix: string, label: string = null) => (
        <label key={suffix} style={{display: 'inline-block', paddingRight: 5}}>
            <input type='checkbox' checked={isFiltered(prefix, suffix)} onChange={() => setFilters(prefix, [suffix], !isFiltered(prefix, suffix))} /> {label || suffix}
        </label>
    );
    const checkboxes = (prefix: string, suffixes: string[]) => suffixes.map(suffix => checkbox(prefix, suffix));

    const radiobox = (prefix: string, suffix: string, label: string) => (
        <label key={suffix}>
            <input type='radio' onChange={() => enableFilter(prefix, suffix)} checked={isFiltered(prefix, suffix)} /> {label}{' '}
        </label>
    );

    const clearFilterLink = (prefix: string) =>
        anyPrefixFiltered(prefix) && (
            <small style={{marginLeft: 'auto'}}>
                <a onClick={() => clearFilters(prefix)}>clear</a>
            </small>
        );

    return (
        <>
            <div className='applications-list__filters__title'>
                FILTERS <i className='fa fa-filter' />
                {anyFiltered && (
                    <small>
                        <a onClick={() => onClearFilter()}>clear all</a>
                    </small>
                )}
                <a onClick={() => setShown(!shown)} style={{marginLeft: 'auto', fontSize: '12px', lineHeight: '5px', display: shown && 'block'}}>
                    {!shown ? 'SHOW' : 'HIDE'}
                </a>
            </div>
            {shown && (
                <div className='applications-list__filters'>
                    <div className='filter'>
                        <div className='filter__header'>
                            OWNERSHIP
                            <HelpIcon
                                title='Always show resources that own/owned by resources that will be shown.
                            For example, if you you want to find what owns pod, so you select "pods" ond choose "Owners"'
                            />
                            {clearFilterLink('ownership')}
                        </div>
                        <div>{checkboxes('ownership', ['Owners', 'Owned'])}</div>
                    </div>
                    <div className='filter'>
                        <div className='filter__header'>SYNC STATUS {clearFilterLink('sync')}</div>
                        <div>{checkboxes('sync', ['Synced', 'OutOfSync'])}</div>
                    </div>
                    <div className='filter'>
                        <div className='filter__header'>HEALTH STATUS {clearFilterLink('health')}</div>
                        <div>{checkboxes('health', ['Healthy', 'Progressing', 'Degraded', 'Suspended', 'Missing', 'Unknown'])}</div>
                    </div>
                    <div className='filter'>
                        <div className='filter__header'>
                            KINDS
                            <small style={{marginLeft: 'auto'}}>
                                <a onClick={() => setFilters('kind', kinds, true)}>all</a> {anyPrefixFiltered('kind') && <a onClick={() => clearFilters('kind')}>clear</a>}
                            </small>
                        </div>
                        <div>{checkboxes('kind', kinds)}</div>
                    </div>
                    {namespaces.length > 1 && (
                        <div className='filter'>
                            <div className='filter__header'>NAMESPACE {clearFilterLink('namespace')}</div>
                            <div>{checkboxes('namespace', namespaces)}</div>
                        </div>
                    )}
                    <div className='filter'>
                        <div className='filter__header'>
                            CREATED WITHIN
                            <HelpIcon title='Use this to find recently created resources, if you want recently synced resources, please raise an issue' />
                            {clearFilterLink('createdWithin')}
                        </div>
                        <div>{[1, 3, 5, 15, 60].map(m => radiobox('createdWithin', String(m), m + 'm'))}</div>
                    </div>
                </div>
            )}
        </>
    );
};
