import {Autocomplete, Checkbox} from 'argo-ui/v2';
import {Tooltip} from 'argo-ui';
import * as React from 'react';

import './filter.scss';

// Upper bound on how many suggestions the autocomplete is handed at once.
const MAX_SUGGESTIONS = 100;

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
    const [prevPropValue, setPrevPropValue] = React.useState(props.value);

    if (prevPropValue !== props.value) {
        setPrevPropValue(props.value);
        setValue(props.value);
    }

    const tooltipProps: Partial<React.ComponentProps<typeof Tooltip>> = {
        placement: 'top',
        popperOptions: {
            modifiers: {
                preventOverflow: {
                    boundariesElement: 'window'
                }
            }
        }
    };

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
            <Tooltip content={<div className='filter__tooltip'>{props.option.label}</div>} {...tooltipProps}>
                <div className='filter__item__label'>{props.option.label}</div>
            </Tooltip>
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
    const [input, setInput] = React.useState('');
    const [collapsed, setCollapsed] = React.useState(props.collapsed || false);
    const options = props.options;

    // The autocomplete mounts every item it is given, so an application with tens of thousands of
    // resources would otherwise put one hidden DOM node per resource on the page. Narrow by what has
    // been typed, then hand over a bounded slice.
    //
    // The autocomplete does its own matching on top of this, and it matches more than a substring of the
    // label: it resolves abbreviations, so "svc" finds Service, it accepts globs, and when nothing
    // matches it falls back to showing the whole list. A plain substring prefilter defeats all three, so
    // leave the list alone for a glob, compare abbreviations as well as labels, and fall back to the head
    // of the list rather than to nothing.
    const labels = React.useMemo(() => {
        const all = props.labels || options.map(o => o.label);
        const needle = input.trim().toLowerCase();
        if (!needle || /[*?[\]]/.test(needle)) {
            return all.slice(0, MAX_SUGGESTIONS);
        }
        const matched = all.filter(label => {
            const candidate = (label || '').toLowerCase();
            const abbreviation = (props.abbreviations?.get(label) || '').toLowerCase();
            return candidate.includes(needle) || abbreviation.includes(needle);
        });
        return (matched.length > 0 ? matched : all).slice(0, MAX_SUGGESTIONS);
    }, [props.labels, options, input, props.abbreviations]);

    const {cleanedValues, selectedKeys} = Object.entries(values).reduce(
        (acc, [key, value]) => {
            if (value !== undefined) {
                acc.cleanedValues[key] = value;
                if (value) {
                    acc.selectedKeys.push(key);
                }
            }
            return acc;
        },
        {cleanedValues: {} as {[label: string]: boolean}, selectedKeys: [] as string[]}
    );

    const valuesNeedCleaning = Object.keys(cleanedValues).length !== Object.keys(values).length;

    // Drop undefined entries from values during render before deriving anything from them.
    if (valuesNeedCleaning) {
        setValues(cleanedValues);
    }

    const tags = props.field
        ? Object.keys(cleanedValues).map(v => {
              if (options?.find(x => x.label === v)) return {label: v, count: options?.find(x => x.label === v).count} as CheckboxOption;
              else return {label: v} as CheckboxOption;
          })
        : [];

    React.useEffect(() => {
        // Sync the selected keys up to the parent. Skip while values still
        // contain undefined entries (a setValues is already queued this render).
        if (!valuesNeedCleaning) {
            props.setSelected(selectedKeys);
        }
    }, [values]);

    const [prevSelectedLength, setPrevSelectedLength] = React.useState(props.selected.length);
    if (prevSelectedLength !== props.selected.length) {
        setPrevSelectedLength(props.selected.length);
        if (props.selected.length === 0) {
            setValues({} as {[label: string]: boolean});
            setInput('');
        }
    }

    return (
        <div className='filter'>
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
