import {ActionButton, Autocomplete, CheckboxOption, CheckboxRow} from 'argo-ui/v2';
import * as React from 'react';
import {SIDEBAR_COLOR, SLATE} from '../../../shared/components';

import './filter.scss';

export interface FilterProps {
    selected: string[];
    setSelected: (items: string[]) => void;
    options?: CheckboxOption[];
    label?: string;
    labels?: string[];
    field?: boolean;
    error?: boolean;
    retry?: () => void;
    loading?: boolean;
    radio?: boolean;
}

const SM_ACTION_BUTTON_STYLE = {marginLeft: 'auto', width: '60px', height: '24px', borderRadius: '3px', marginRight: 0};

export const FiltersGroup = (props: {children?: React.ReactNode; content: React.ReactNode; appliedFilter?: string[]; onClearFilter?: () => void}) => {
    return (
        <div className='filters-group'>
            <div style={{display: 'flex', marginBottom: '1em'}}>
                FILTERS
                <div style={{marginLeft: 'auto'}}>
                    {props.appliedFilter?.length > 0 && props.onClearFilter ? (
                        <ActionButton action={() => props.onClearFilter()} label='CLEAR ALL' style={SM_ACTION_BUTTON_STYLE} />
                    ) : (
                        <i className='fa fa-filter' />
                    )}
                </div>
            </div>
            <>{props.children}</>
            <div>{props.content}</div>
        </div>
    );
};

export const Filter = (props: FilterProps) => {
    const init = {} as {[label: string]: boolean};
    props.selected.forEach(s => (init[s] = true));

    const [values, setValues] = React.useState(init);
    const [tags, setTags] = React.useState([]);
    const [input, setInput] = React.useState('');
    const [collapsed, setCollapsed] = React.useState(false);

    const labels = props.labels || props.options.map(o => o.label);

    React.useEffect(() => {
        const map: string[] = Object.keys(values).filter(s => values[s]);
        props.setSelected(map);
        if (props.field) {
            setTags(
                Object.keys(values).map(v => {
                    return {label: v} as CheckboxOption;
                })
            );
        }
    }, [values]);

    React.useEffect(() => {
        if (props.selected.length === 0) {
            setValues({} as {[label: string]: boolean});
            setInput('');
        }
    }, [props.selected.length]);

    return (
        <div className='filter'>
            <div className='filter__header'>
                {props.label || 'FILTER'}
                {(props.selected || []).length > 0 || (props.field && Object.keys(values).length > 0) ? (
                    <ActionButton
                        action={() => {
                            setValues({} as {[label: string]: boolean});
                            setInput('');
                        }}
                        label='CLEAR'
                        style={SM_ACTION_BUTTON_STYLE}
                        dark
                    />
                ) : (
                    <i
                        className={`fa fa-caret-${collapsed ? 'down' : 'up'} filter__collapse`}
                        onClick={() => setCollapsed(!collapsed)}
                        style={{marginLeft: 'auto', color: 'white'}}
                    />
                )}
            </div>
            {!collapsed &&
                (props.loading ? (
                    <FilterLoading />
                ) : props.error ? (
                    <FilterError retry={props.retry} />
                ) : (
                    <React.Fragment>
                        {props.field && (
                            <Autocomplete
                                placeholder={props.label}
                                items={labels}
                                value={input}
                                onChange={e => setInput(e.target.value)}
                                onItemClick={val => {
                                    const update = {...values};
                                    update[val ? val : input] = true;
                                    setInput('');
                                    setValues(update);
                                }}
                                style={{width: '100%'}}
                                inputStyle={{marginBottom: '0.5em', backgroundColor: 'white'}}
                            />
                        )}
                        {((props.field ? tags : props.options) || []).map((opt, i) => (
                            <CheckboxRow
                                key={i}
                                value={values[opt.label]}
                                onChange={val => {
                                    const update = props.radio && val ? {} : {...values};
                                    update[opt.label] = val;
                                    setValues(update);
                                }}
                                option={opt}
                                style={{backgroundColor: SIDEBAR_COLOR, border: `1px solid ${SLATE}`}}
                                selectedStyle={{backgroundColor: SLATE, color: 'white', border: `1px solid ${SLATE}`}}
                            />
                        ))}
                    </React.Fragment>
                ))}
        </div>
    );
};

const FilterError = (props: {retry: () => void}) => (
    <div className='filter__error'>
        <i className='fa fa-exclamation-circle' /> ERROR LOADING FILTER
        <div onClick={() => props.retry()} className='filter__error__retry'>
            <i className='fa fa-redo' /> RETRY
        </div>
    </div>
);

const FilterLoading = () => (
    <div className='filter__loading'>
        <i className='fa fa-circle-notch fa-spin' /> LOADING
    </div>
);
