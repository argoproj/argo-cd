import {Autocomplete, CheckboxOption, CheckboxRow} from 'argo-ux';
import * as React from 'react';

import './filter.scss';

interface FilterMap {
    [label: string]: boolean;
}

export const Filter = (props: {selected: string[]; setSelected: (items: string[]) => void; options?: CheckboxOption[]; label?: string; field?: boolean}) => {
    const init = {} as FilterMap;
    props.selected.forEach(s => (init[s] = true));

    const [values, setValues] = React.useState(init);
    const [tags, setTags] = React.useState([]);
    const [input, setInput] = React.useState('');
    const [collapsed, setCollapsed] = React.useState(false);

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

    return (
        <div className='filter'>
            <div className='filter__header'>
                {props.label || 'FILTER'}
                {(props.selected || []).length > 0 || (props.field && Object.keys(values).length > 0) ? (
                    <div
                        className='argo-button argo-button--base argo-button--sm'
                        style={{marginLeft: 'auto'}}
                        onClick={() => {
                            setValues({} as FilterMap);
                        }}>
                        <i className='fa fa-times-circle' /> CLEAR
                    </div>
                ) : (
                    <i className={`fa fa-caret-${collapsed ? 'down' : 'up'} filter__collapse`} onClick={() => setCollapsed(!collapsed)} />
                )}
            </div>
            {!collapsed && (
                <React.Fragment>
                    {props.field && (
                        <Autocomplete
                            placeholder={props.label}
                            items={(props.options || []).map(opt => {
                                return opt.label;
                            })}
                            value={input}
                            onChange={e => setInput(e.target.value)}
                            onItemClick={val => {
                                const update = {...values};
                                update[val] = true;
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
                                const update = {...values};
                                update[opt.label] = val;
                                setValues(update);
                            }}
                            option={opt}
                        />
                    ))}
                </React.Fragment>
            )}
        </div>
    );
};
