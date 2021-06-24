import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {useContext, useState} from 'react';
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

    if (!shown) {
        return (
            <a className='fa fa-pull-right argo-button argo-button--base-o' onClick={() => setShown(true)}>
                <i className='fa fa-filter' />
            </a>
        );
    }

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

    const labels: {[name: string]: string[]} = {};
    tree.nodes.forEach(x =>
        Object.entries(x.labels || {}).forEach(([name, value]) => {
            labels[name] = (labels[name] || [])
                .concat(value)
                .filter(uniq)
                .sort();
        })
    );

    const [expandedLabelName, setExpandedLabelNames] = useState([]);

    const checkbox = (prefix: string, suffix: string, label: string = null) => (
        <label key={suffix} style={{display: 'inline-block', paddingRight: 5}}>
            <input type='checkbox' checked={isFiltered(prefix, suffix)} onChange={() => setFilters(prefix, [suffix], !isFiltered(prefix, suffix))} /> {label || suffix}{' '}
        </label>
    );
    const checkboxes = (prefix: string, suffixes: string[]) => suffixes.map(suffix => checkbox(prefix, suffix));

    const radiobox = (prefix: string, suffix: string, label?: string) => (
        <label key={suffix}>
            <input type='radio' onChange={() => enableFilter(prefix, suffix)} checked={isFiltered(prefix, suffix)} /> {label || suffix}{' '}
        </label>
    );

    const clearFilterLink = (prefix: string) =>
        anyPrefixFiltered(prefix) && (
            <small>
                <a onClick={() => clearFilters(prefix)}>clear</a>
            </small>
        );

    return (
        <div className='white-box'>
            <div className='row'>
                <div className='columns small-12 large-12'>
                    <h4>
                        <i className='fa fa-filter' />
                        {anyFiltered && (
                            <small>
                                <a onClick={() => onClearFilter()}>clear all</a>
                            </small>
                        )}
                        <span className='fa-pull-right'>
                            <a onClick={() => setShown(false)}>
                                <i className='fa fa-times' />
                            </a>
                        </span>
                    </h4>
                </div>
            </div>
            <div className='row'>
                <div className='columns large-4'>
                    <div>
                        <h5>
                            Ownership
                            <HelpIcon
                                title='Always show resources that own/owned by resources that will be shown.
                            For example, if you you want to find what owns pod, so you select "pods" ond choose "Owners"'
                            />{' '}
                            {clearFilterLink('ownership')}
                        </h5>
                        <p>Always show: {checkboxes('ownership', ['Owners', 'Owned'])}</p>
                    </div>
                    <div>
                        <h5>Sync status {clearFilterLink('sync')}</h5>
                        <p>{checkboxes('sync', ['Synced', 'OutOfSync'])}</p>
                    </div>
                    <div>
                        <h5>Health status {clearFilterLink('health')}</h5>
                        <p>{checkboxes('health', ['Healthy', 'Progressing', 'Degraded', 'Suspended', 'Missing', 'Unknown'])}</p>
                    </div>
                </div>
                <div className='columns large-4'>
                    <div>
                        <h5>
                            Kinds{' '}
                            <small>
                                <a onClick={() => setFilters('kind', kinds, true)}>all</a>
                            </small>{' '}
                            {clearFilterLink('kind')}
                        </h5>
                        <p>{checkboxes('kind', kinds)}</p>
                    </div>
                    {namespaces.length > 1 && (
                        <div>
                            <h5>Namespaces {clearFilterLink('namespace')}</h5>
                            <p>{checkboxes('namespace', namespaces)}</p>
                        </div>
                    )}
                    <div>
                        <h5>
                            Created within
                            <HelpIcon title='Use this to find recently created resources, if you want recently synced resources, please raise an issue' />{' '}
                            {clearFilterLink('createdWithin')}
                        </h5>
                        <p>{[1, 3, 5, 15, 60].map(m => radiobox('createdWithin', String(m), m + 'm'))}</p>
                    </div>
                </div>
                <div className='columns large-4'>
                    <div>
                        <h5>
                            Labels
                            <HelpIcon title='Only the labels of live resources are included, please raise an issue if you want target resource labels too' />{' '}
                            {clearFilterLink('label')}
                        </h5>
                        {Object.entries(labels).map(([name, values]) => (
                            <div key={name}>
                                {!expandedLabelName.includes(name) ? (
                                    <>
                                        <a onClick={() => setExpandedLabelNames(expandedLabelName.concat(name))}>
                                            <i className='fa fa-chevron-right' /> {name}
                                        </a>
                                        <br />
                                    </>
                                ) : (
                                    <>
                                        <a onClick={() => setExpandedLabelNames(expandedLabelName.filter(x => x !== name))}>
                                            <i className='fa fa-chevron-down' /> {name}
                                        </a>
                                        <br />
                                        {values.map(value => (
                                            <div style={{paddingLeft: 20}} key={value}>
                                                {' '}
                                                {checkbox('label', name + '=' + value, value)}
                                                <br />
                                            </div>
                                        ))}
                                    </>
                                )}
                            </div>
                        ))}
                        {Object.keys(labels).length === 0 && <p>No labels to filter on, trying syncing your app</p>}
                    </div>
                </div>
            </div>
        </div>
    );
};
