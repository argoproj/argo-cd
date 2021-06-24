import {Checkbox} from 'argo-ui/src/index';
import * as React from 'react';
import {useContext} from 'react';
import {Context} from '../../../shared/context';
import {ApplicationTree} from '../../../shared/models';
import {AppDetailsPreferences, services} from '../../../shared/services';

const syncStatuses = ['Synced', 'OutOfSync', 'Unknown'];
const healthStatues = ['Healthy', 'Degraded', 'Missing', 'Unknown'];
const uniq = (value: string, index: number, self: string[]) => self.indexOf(value) === index;

export const Filters = ({pref, tree}: {pref: AppDetailsPreferences; tree: ApplicationTree}) => {
    const {navigation} = useContext(Context);

    const isIncluded = (prefix: string, suffix: string) => pref.resourceFilter.includes(`${prefix}:${suffix}`);
    const setIncluded = (prefix: string, suffixes: string[], v: boolean) => {
        const filters = suffixes.map(suffix => `${prefix}:${suffix}`);
        const items = pref.resourceFilter.filter(y => !filters.includes(y));
        if (v) {
            items.push(...filters);
        }
        navigation.goto('.', {resource: items.join(',')});
        services.viewPreferences.updatePreferences({appDetails: {...pref, resourceFilter: items}});
    };
    // this is smarter than it looks at first glance, rather than just un-checked known items,
    // it instead finds out what is enabled, and then removes them, which will be tolerant to weird or unkown items
    const clear = (prefix: string) => setIncluded(prefix, pref.resourceFilter.filter(v => v.startsWith(prefix + ':')).map(v => v.replace(prefix + ':', '')), false);

    const kinds = tree.nodes
        .map(x => x.kind)
        .filter(uniq)
        .sort();
    const namespaces = tree.nodes
        .map(x => x.namespace)
        .filter(uniq)
        .sort();

    const checkbox = (prefix: string, suffix: string) => (
        <label>
            <Checkbox checked={isIncluded(prefix, suffix)} onChange={v => setIncluded(prefix, [suffix], v)} /> {suffix}{' '}
        </label>
    );
    // three checkboxes per row, stops the checkbox being on one line, and the label on the next
    const checkboxes = (prefix: string, suffixes: string[]) =>
        suffixes.map((suffix, i) => (
            <React.Fragment key={suffix}>
                {checkbox(prefix, suffix)}
                {i % 3 === 2 && <br />}
            </React.Fragment>
        ));

    return (
        <>
            <div>
                <h5>
                    Sync{' '}
                    <small>
                        <a onClick={() => clear('sync')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('sync', syncStatuses)}</p>
            </div>
            <div>
                <h5>
                    Health{' '}
                    <small>
                        <a onClick={() => clear('health')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('health', healthStatues)}</p>
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
            <div>
                <h5>
                    Namespaces{' '}
                    <small>
                        <a onClick={() => clear('namespace')}>clear</a>
                    </small>
                </h5>
                <p>{checkboxes('namespace', namespaces)}</p>
            </div>
        </>
    );
};
