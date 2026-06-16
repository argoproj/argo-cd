import {Checkbox, HelpIcon, Select} from 'argo-ui';
import * as React from 'react';
import * as models from '../../../shared/models';
import './application-selective-sync.scss';

const OPERATORS = ['In', 'NotIn', 'Exists', 'DoesNotExist'];

// isSelectiveSyncEnabled returns true when automated sync is restricted to a subset of resources.
export function isSelectiveSyncEnabled(automated?: models.Automated): boolean {
    return !!(automated && automated.selective && automated.selective.enabled);
}

const valuesToText = (values?: string[]) => (values || []).join(', ');
const textToValues = (text: string) =>
    text
        .split(',')
        .map(v => v.trim())
        .filter(v => v !== '');

// SelectiveSyncEditor renders a structured editor for the spec.syncPolicy.automated.selective field.
export const SelectiveSyncEditor = (props: {selective?: models.SelectiveSync; onChange: (selective: models.SelectiveSync) => void}) => {
    const selective = props.selective || {};
    const filters = selective.filters || [];
    const matchExpressions = selective.matchExpressions || [];

    const update = (patch: Partial<models.SelectiveSync>) => props.onChange({...selective, ...patch});

    return (
        <div className='application-selective-sync'>
            <div className='checkbox-container'>
                <Checkbox id='selectiveSyncEnabled' checked={!!selective.enabled} onChange={val => update({enabled: val})} />
                <label htmlFor='selectiveSyncEnabled'>Enable Selective Sync</label>
                <HelpIcon title='If checked, auto-sync only applies to the resources selected below. Leave both lists empty to auto-sync all resources.' />
            </div>
            {selective.enabled && (
                <React.Fragment>
                    <div className='application-selective-sync__section'>
                        <label className='application-selective-sync__label'>
                            Filters{' '}
                            <HelpIcon title='Select resources by group/kind/name/namespace. An empty field (or "*") matches any value. A resource matches when it matches at least one filter.' />
                        </label>
                        {filters.map((filter, i) => (
                            <div className='row application-selective-sync__row' key={`filter-${i}`}>
                                <div className='columns small-3'>
                                    <input
                                        className='argo-field'
                                        placeholder='group'
                                        value={filter.group || ''}
                                        onChange={e => {
                                            const next = filters.slice();
                                            next[i] = {...filter, group: e.target.value};
                                            update({filters: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-3'>
                                    <input
                                        className='argo-field'
                                        placeholder='kind'
                                        value={filter.kind || ''}
                                        onChange={e => {
                                            const next = filters.slice();
                                            next[i] = {...filter, kind: e.target.value};
                                            update({filters: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-3'>
                                    <input
                                        className='argo-field'
                                        placeholder='name'
                                        value={filter.name || ''}
                                        onChange={e => {
                                            const next = filters.slice();
                                            next[i] = {...filter, name: e.target.value};
                                            update({filters: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-2'>
                                    <input
                                        className='argo-field'
                                        placeholder='namespace'
                                        value={filter.namespace || ''}
                                        onChange={e => {
                                            const next = filters.slice();
                                            next[i] = {...filter, namespace: e.target.value};
                                            update({filters: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-1'>
                                    <i
                                        className='fa fa-times-circle application-selective-sync__remove'
                                        title='Remove filter'
                                        onClick={() => update({filters: filters.filter((_, j) => j !== i)})}
                                    />
                                </div>
                            </div>
                        ))}
                        <button type='button' className='argo-button argo-button--base-o' onClick={() => update({filters: filters.concat([{}])})}>
                            <i className='fa fa-plus' /> Add filter
                        </button>
                    </div>

                    <div className='application-selective-sync__section'>
                        <label className='application-selective-sync__label'>
                            Match Expressions{' '}
                            <HelpIcon title='Select resources by their labels, using Kubernetes label selector requirements. Values are comma-separated and ignored for the Exists/DoesNotExist operators.' />
                        </label>
                        {matchExpressions.map((expr, i) => (
                            <div className='row application-selective-sync__row' key={`expr-${i}`}>
                                <div className='columns small-4'>
                                    <input
                                        className='argo-field'
                                        placeholder='label key'
                                        value={expr.key || ''}
                                        onChange={e => {
                                            const next = matchExpressions.slice();
                                            next[i] = {...expr, key: e.target.value};
                                            update({matchExpressions: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-3'>
                                    <Select
                                        value={expr.operator || 'In'}
                                        options={OPERATORS}
                                        onChange={opt => {
                                            const next = matchExpressions.slice();
                                            next[i] = {...expr, operator: opt.value};
                                            update({matchExpressions: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-4'>
                                    <input
                                        className='argo-field'
                                        placeholder='values (comma-separated)'
                                        disabled={expr.operator === 'Exists' || expr.operator === 'DoesNotExist'}
                                        value={valuesToText(expr.values)}
                                        onChange={e => {
                                            const next = matchExpressions.slice();
                                            next[i] = {...expr, values: textToValues(e.target.value)};
                                            update({matchExpressions: next});
                                        }}
                                    />
                                </div>
                                <div className='columns small-1'>
                                    <i
                                        className='fa fa-times-circle application-selective-sync__remove'
                                        title='Remove match expression'
                                        onClick={() => update({matchExpressions: matchExpressions.filter((_, j) => j !== i)})}
                                    />
                                </div>
                            </div>
                        ))}
                        <button
                            type='button'
                            className='argo-button argo-button--base-o'
                            onClick={() => update({matchExpressions: matchExpressions.concat([{key: '', operator: 'In', values: []}])})}>
                            <i className='fa fa-plus' /> Add match expression
                        </button>
                    </div>
                </React.Fragment>
            )}
        </div>
    );
};

// SelectiveSyncView renders a read-only summary of the configured selective sync selection.
export const SelectiveSyncView = (props: {selective?: models.SelectiveSync}) => {
    const selective = props.selective;
    if (!selective || !selective.enabled) {
        return null;
    }
    const filters = selective.filters || [];
    const matchExpressions = selective.matchExpressions || [];
    if (filters.length === 0 && matchExpressions.length === 0) {
        return <div className='application-selective-sync__view'>All resources (no filters or match expressions configured)</div>;
    }
    return (
        <div className='application-selective-sync__view'>
            {filters.length > 0 && (
                <div>
                    <span className='application-selective-sync__view-title'>Filters:</span>
                    <ul>
                        {filters.map((f, i) => (
                            <li key={`filter-${i}`}>{[f.group || '*', f.kind || '*', f.name || '*', f.namespace || '*'].join(' / ')}</li>
                        ))}
                    </ul>
                </div>
            )}
            {matchExpressions.length > 0 && (
                <div>
                    <span className='application-selective-sync__view-title'>Match Expressions:</span>
                    <ul>
                        {matchExpressions.map((expr, i) => (
                            <li key={`expr-${i}`}>
                                {expr.key} {expr.operator} {valuesToText(expr.values) && `[${valuesToText(expr.values)}]`}
                            </li>
                        ))}
                    </ul>
                </div>
            )}
        </div>
    );
};
