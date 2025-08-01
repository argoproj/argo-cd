import {Autocomplete, Checkbox} from 'argo-ui/v2';
import * as React from 'react';

import './filter.scss';

interface FilterProps {
    selected: string[];
    setSelected: (items: string[]) => void;
    options?: CheckboxOption[];
    label?: string;
    labels?: string[];
    abbreviations?: Map<string, string>;
    field?: boolean;
    error?: boolean;
    retry?: () => void;
    loading?: boolean;
    radio?: boolean;
    collapsed?: boolean;
}

export interface CheckboxOption {
    label: string;
    count?: number;
    icon?: React.ReactNode;
}

export const CheckboxRow = (props: {value: boolean; onChange?: (value: boolean) => void; option: CheckboxOption}) => {
    const [value, setValue] = React.useState(props.value);

    React.useEffect(() => {
        setValue(props.value);
    }, [props.value]);

    return (
        <div className={`filter__item ${value ? 'filter__item--selected' : ''}`} onClick={() => setValue(!value)}>
            <Checkbox
                onChange={val => {
                    setValue(val);
                    if (props.onChange) {
                        props.onChange(val);
                    }
                }}
                value={value}
                style={{
                    marginRight: '8px'
                }}
            />
            {props.option.icon && <div style={{marginRight: '5px'}}>{props.option.icon}</div>}
            <div className='filter__item__label'>{props.option.label}</div>
            <div style={{marginLeft: 'auto'}}>{props.option.count}</div>
        </div>
    );
};

export const FiltersGroup = (props: {
    children?: React.ReactNode;
    content: React.ReactNode;
    appliedFilter?: string[];
    onClearFilter?: () => void;
    collapsed?: boolean;
    title?: string;
}) => {
    return (
        !props.collapsed && (
            <div className='filters-group'>
                {props.title && <div className='filters-group__title'>{props.title}</div>}
                {props.appliedFilter?.length > 0 && props.onClearFilter && (
                    <div className='filters-group__header'>
                        <button onClick={() => props.onClearFilter()} className='argo-button argo-button--base argo-button--sm'>
                            <i className='fa fa-times-circle' /> CLEAR ALL
                        </button>
                    </div>
                )}
                {props.children}
                <div className='filters-group__content'>{props.content}</div>
            </div>
        )
    );
};

export const Filter = (props: FilterProps) => {
    const init = {} as {[label: string]: boolean};
    props.selected.forEach(s => (init[s] = true));

    const [values, setValues] = React.useState(init);
    const [tags, setTags] = React.useState([]);
    const [input, setInput] = React.useState('');
    const [collapsed, setCollapsed] = React.useState(props.collapsed || false);
    const [options, setOptions] = React.useState(props.options);

    React.useEffect(() => {
        setOptions(props.options);
    }, [props.options]);

    const labels = props.labels || options.map(o => o.label);

    React.useEffect(() => {
        const map: string[] = Object.keys(values).filter(s => values[s]);
        props.setSelected(map);
        if (props.field) {
            setTags(
                Object.keys(values).map(v => {
                    if (options?.find(x => x.label === v)) return {label: v, count: options?.find(x => x.label === v).count} as CheckboxOption;
                    else return {label: v} as CheckboxOption;
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

    const totalCount = options.reduce((countSum, option) => {
        return countSum + option.count;
    }, 0);

    return (
        <div className='filter' key={totalCount + props.label}>
            <div className='filter__header'>
                <span className='filter__header__label' title={props.label || 'FILTER'}>
                    {props.label || 'FILTER'}
                </span>
                {(props.selected || []).length > 0 || (props.field && Object.keys(values).length > 0) ? (
                    <button
                        className='argo-button argo-button--base argo-button--sm argo-button--right'
                        onClick={() => {
                            setValues({} as {[label: string]: boolean});
                            setInput('');
                        }}>
                        <i className='fa fa-times-circle' /> CLEAR
                    </button>
                ) : (
                    <i className={`fa fa-caret-${collapsed ? 'down' : 'up'} filter__collapse`} onClick={() => setCollapsed(!collapsed)} />
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
                                abbreviations={props.abbreviations}
                                value={input}
                                onChange={e => setInput(e.target.value)}
                                onItemClick={val => {
                                    const update = {...values};
                                    update[val ? val : input] = true;
                                    setInput('');
                                    setValues(update);
                                }}
                                style={{width: '100%'}}
                                inputStyle={{marginBottom: '0.5em', backgroundColor: 'black', border: 'none', color: '#fff'}}
                            />
                        )}
                        {((props.field ? tags : options) || []).map((opt, i) => (
                            <CheckboxRow
                                key={i}
                                value={values[opt.label]}
                                onChange={val => {
                                    const update = props.radio && val ? {} : {...values};
                                    update[opt.label] = val;
                                    setValues(update);
                                }}
                                option={opt}
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
