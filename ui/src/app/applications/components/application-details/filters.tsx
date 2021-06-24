import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {useState} from 'react';
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
    const isIncluded = (prefix: string, suffix: string) => pref.resourceFilter.includes(`${prefix}:${suffix}`);
    const setIncluded = (prefix: string, suffixes: string[], v: boolean) => {
        const filters = suffixes.map(suffix => `${prefix}:${suffix}`);
        const items = pref.resourceFilter.filter(y => !filters.includes(y));
        if (v) {
            items.push(...filters);
        }
        onSetFilter(items);
    };
    // this is smarter than it looks at first glance, rather than just un-checked known items,
    // it instead finds out what is enabled, and then removes them, which will be tolerant to weird or unknown items
    const clear = (prefix: string) => setIncluded(prefix, pref.resourceFilter.filter(v => v.startsWith(prefix + ':')).map(v => v.replace(prefix + ':', '')), false);

    // we need to include ones that might have been filter in other apps that do not apply to the current app,
    // otherwise the user will not be able to clear them from this panel
    const alreadyFilteredOn = (prefix: string) => pref.resourceFilter.filter(v => v.startsWith(prefix + ':')).map(v => v.replace(prefix + ':', ''));

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
            <input type='checkbox' checked={isIncluded(prefix, suffix)} onChange={() => setIncluded(prefix, [suffix], !isIncluded(prefix, suffix))} /> {label || suffix}{' '}
        </label>
    );
    const checkboxes = (prefix: string, suffixes: string[]) => suffixes.map(suffix => checkbox(prefix, suffix));

    const radiobox = (prefix: string, suffix: string, label?: string) => (
        <label key={suffix}>
            <input
                type='radio'
                onChange={() => {
                    clear(prefix);
                    setIncluded(prefix, [suffix], true);
                }}
                checked={isIncluded(prefix, suffix)}
            />{' '}
            {label || suffix}
        </label>
    );

    return (
        <div style={{overflow: 'scroll'}}>
            <h4>
                <i className='fa fa-filter' />
                Filters{' '}
                <small>
                    <a onClick={() => onClearFilter()}>clear all</a>
                </small>
            </h4>
            <hr />
            <div>
                <h5>
                    Ownership
                    <HelpIcon title='Always show resources that own/owned by resources that will be show. E.g. you want to find who owns pod, so you select "pods" ond choose "Owners"' />{' '}
                    <small>
                        <a onClick={() => clear('ownership')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('ownership', ['Owners', 'Owns'])}</p>
            </div>
            <hr />
            <div>
                <h5>
                    Sync{' '}
                    <small>
                        <a onClick={() => clear('sync')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('sync', ['Synced', 'OutOfSync', 'Unknown'])}</p>
            </div>
            <div>
                <h5>
                    Health{' '}
                    <small>
                        <a onClick={() => clear('health')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('health', ['Healthy', 'Degraded', 'Missing', 'Unknown'])}</p>
            </div>
            <div>
                <h5>
                    Kinds{' '}
                    <small>
                        <a onClick={() => setIncluded('kind', kinds, true)}>all</a> <a onClick={() => clear('kind')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('kind', kinds)}</p>
            </div>
            {namespaces.length > 1 && (
                <div>
                    <h5>
                        Namespaces{' '}
                        <small>
                            <a onClick={() => clear('namespace')}>clear</a>
                        </small>
                    </h5>
                    <p>{checkboxes('namespace', namespaces)}</p>
                </div>
            )}
            <div>
                <h5>
                    Created within{' '}
                    <small>
                        <a onClick={() => clear('createdWithin')}>clear</a>
                    </small>
                </h5>
                <p>{[1, 3, 5, 15, 60].map(m => radiobox('createdWithin', String(m), m + 'm'))}</p>
            </div>
            <div>
                <h5>
                    Labels
                    <HelpIcon title='Only the labels of live resources are included, please raise an issue if you want target resource labels too' />{' '}
                    <small>
                        <a onClick={() => clear('label')}>clear</a>
                    </small>
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
            </div>
        </div>
    );
};
