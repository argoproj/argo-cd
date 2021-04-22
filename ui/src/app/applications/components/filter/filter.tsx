import {Autocomplete} from 'argo-ui';
import * as React from 'react';

import './filter.scss';

interface FilterMap {
    [label: string]: boolean;
}

interface FilterOption {
    label: string;
    count?: number;
    icon?: React.ReactNode;
}

const FilterRow = (props: {init: boolean; onChange?: (value: boolean) => void; option: FilterOption}) => {
    const [value, setValue] = React.useState(props.init);

    React.useEffect(() => {
        setValue(props.init);
    }, [props.init]);

    return (
        <div
            className={`filter__item ${value ? 'filter__item--selected' : ''}`}
            onClick={() => {
                setValue(!value);
                if (props.onChange) {
                    props.onChange(!value);
                }
            }}>
            <i className={`${value ? 'fas fa-check-square' : 'fa fa-square'}`} style={{marginRight: '8px'}} />
            {props.option.icon && <div style={{marginRight: '5px'}}>{props.option.icon}</div>}
            <div className='filter__item__label'>{props.option.label}</div>
            <div style={{marginLeft: 'auto'}}>{props.option.count || 0}</div>
        </div>
    );
};

export const Filter = (props: {selected: string[]; setSelected: (items: string[]) => void; options?: FilterOption[]; label?: string; field?: boolean}) => {
    const init = {} as FilterMap;
    props.selected.forEach(s => (init[s] = true));

    const [values, setValues] = React.useState(init);
    const [tags, setTags] = React.useState([]);
    const [input, setInput] = React.useState('');

    React.useEffect(() => {
        const map: string[] = Object.keys(values).filter(s => values[s]);
        props.setSelected(map);
        if (props.field) {
            setTags(
                Object.keys(values).map(v => {
                    return {label: v} as FilterOption;
                })
            );
        }
    }, [values]);

    return (
        <div className='filter'>
            <div className='filter__header'>
                {props.label || 'FILTER'}
                {(props.selected || []).length > 0 && (
                    <div className='argo-button argo-button--base argo-button--sm' style={{marginLeft: 'auto'}} onClick={() => setValues({} as FilterMap)}>
                        <i className='fa fa-times-circle' /> CLEAR
                    </div>
                )}
            </div>
            {props.field && (
                <Autocomplete
                    items={(props.options || []).map(opt => {
                        return {value: opt.label};
                    })}
                    value={input}
                    onChange={e => setInput(e.target.value)}
                    filterSuggestions={true}
                    onSelect={val => {
                        const update = {...values};
                        update[val] = true;
                        setValues(update);
                    }}
                    inputProps={{style: {marginBottom: '0.5em'}}}
                />
            )}
            {((props.field ? tags : props.options) || []).map((opt, i) => (
                <FilterRow
                    key={i}
                    init={values[opt.label]}
                    onChange={val => {
                        const update = {...values};
                        update[opt.label] = val;
                        setValues(update);
                    }}
                    option={opt}
                />
            ))}
        </div>
    );
};
