import * as React from 'react';
import * as ReactForm from 'react-form';
import {FormValue} from 'react-form';

/*
    This provide a way to may a form field to an array of items. It allows you to

    * Add a new (maybe duplicate) item.
    * Replace an item.
    * Remove an item.

    E.g.
    env:
    - name: FOO
      value: bar
    - name: BAZ
      value: qux
    # You can have dup items
    - name: FOO
      value: bar

    It does not allow re-ordering of elements (maybe in a v2).
 */

export interface NameValue {
    name: string;
    value: string;
}

export const NameValueEditor = (item: NameValue, onChange?: (item: NameValue) => any) => {
    return (
        <React.Fragment>
            <input
                // disable chrome autocomplete
                autoComplete='fake'
                className='argo-field'
                style={{width: '40%', borderColor: !onChange ? '#eff3f5' : undefined}}
                placeholder='Name'
                value={item.name}
                onChange={e => onChange({...item, name: e.target.value})}
                // onBlur={e=>onChange({...item, name: e.target.value})}
                title='Name'
                readOnly={!onChange}
            />
            &nbsp; = &nbsp;
            <input
                // disable chrome autocomplete
                autoComplete='fake'
                className='argo-field'
                style={{width: '40%', borderColor: !onChange ? '#eff3f5' : undefined}}
                placeholder='Value'
                value={item.value || ''}
                onChange={e => onChange({...item, value: e.target.value})}
                title='Value'
                readOnly={!onChange}
            />
            &nbsp;
        </React.Fragment>
    );
};

export const ValueEditor = (item: string, onChange: (item: string) => any) => {
    return (
        <input
            // disable chrome autocomplete
            autoComplete='fake'
            className='argo-field'
            style={{width: '40%', borderColor: !onChange ? '#eff3f5' : undefined}}
            placeholder='Value'
            value={item || ''}
            onChange={e => onChange(e.target.value)}
            title='Value'
            readOnly={!onChange}
        />
    );
};

interface Props<T> {
    items: T[];
    onChange: (items: T[]) => void;
    editor: (item: T, onChange: (updated: T) => any) => React.ReactNode;
}

export function ArrayInput<T>(props: Props<T>) {
    const addItem = (item: T) => {
        props.onChange([...props.items, item]);
    };

    const replaceItem = (item: T, i: number) => {
        const items = props.items.slice();
        items[i] = item;
        props.onChange(items);
    };

    const removeItem = (i: number) => {
        const items = props.items.slice();
        items.splice(i, 1);
        props.onChange(items);
    };

    return (
        <div className='argo-field' style={{border: 0, marginTop: '15px', zIndex: 1}}>
            {props.items.map((item, i) => (
                <div key={`item-${i}`} style={{marginBottom: '5px'}}>
                    {props.editor(item, (updated: T) => replaceItem(updated, i))}
                    &nbsp;
                    <button>
                        <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => removeItem(i)} />
                    </button>{' '}
                </div>
            ))}
            {props.items.length === 0 && <label>No items</label>}
            <div>
                <button className='argo-button argo-button--base argo-button--short' onClick={() => addItem({} as T)}>
                    <i style={{cursor: 'pointer'}} className='fa fa-plus' />
                </button>
            </div>
        </div>
    );
}

export const ResetOrDeleteButton = (props: {
    isPluginPar: boolean;
    getValue: () => FormValue;
    name: string;
    index: number;
    setValue: (value: FormValue) => void;
    setAppParamsDeletedState: any;
}) => {
    const handleDeleteChange = () => {
        if (props.index >= 0) {
            props.setAppParamsDeletedState((val: string[]) => val.concat(props.name));
        }
    };

    const handleResetChange = () => {
        if (props.index >= 0) {
            const items = [...props.getValue()];
            items.splice(props.index, 1);
            props.setValue(items);
        }
    };

    const disabled = props.index === -1;

    const content = props.isPluginPar ? 'Reset' : 'Delete';
    let tooltip = '';
    if (content === 'Reset' && !disabled) {
        tooltip = 'Resets the parameter to the value provided by the plugin. This removes the parameter override from the application manifest';
    } else if (content === 'Delete' && !disabled) {
        tooltip = 'Deletes this parameter values from the application manifest.';
    }

    return (
        <button
            className='argo-button argo-button--base'
            disabled={disabled}
            title={tooltip}
            style={{fontSize: '12px', display: 'flex', marginLeft: 'auto', marginTop: '8px'}}
            onClick={props.isPluginPar ? handleResetChange : handleDeleteChange}>
            {content}
        </button>
    );
};

export const ArrayInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    return <ArrayInput editor={NameValueEditor} items={getValue() || []} onChange={setValue} />;
});

export const ArrayValueField = ReactForm.FormField(
    (props: {fieldApi: ReactForm.FieldApi; name: string; defaultVal: string[]; isPluginPar: boolean; setAppParamsDeletedState: any}) => {
        const {
            fieldApi: {getValue, setValue}
        } = props;

        let liveParamArray;
        const liveParam = getValue()?.find((val: {name: string; array: object}) => val.name === props.name);
        if (liveParam) {
            liveParamArray = liveParam?.array ?? [];
        }
        const index = getValue()?.findIndex((val: {name: string; array: object}) => val.name === props.name) ?? -1;
        const values = liveParamArray ?? props.defaultVal ?? [];

        return (
            <React.Fragment>
                <ResetOrDeleteButton
                    isPluginPar={props.isPluginPar}
                    getValue={getValue}
                    name={props.name}
                    index={index}
                    setValue={setValue}
                    setAppParamsDeletedState={props.setAppParamsDeletedState}
                />
                <ArrayInput
                    editor={ValueEditor}
                    items={values || []}
                    onChange={change => {
                        const update = change.map((val: string | object) => (typeof val !== 'string' ? '' : val));
                        if (index >= 0) {
                            getValue()[index].array = update;
                            setValue([...getValue()]);
                        } else {
                            setValue([...(getValue() || []), {name: props.name, array: update}]);
                        }
                    }}
                />
            </React.Fragment>
        );
    }
);

export const StringValueField = ReactForm.FormField(
    (props: {fieldApi: ReactForm.FieldApi; name: string; defaultVal: string; isPluginPar: boolean; setAppParamsDeletedState: any}) => {
        const {
            fieldApi: {getValue, setValue}
        } = props;
        let liveParamString;
        const liveParam = getValue()?.find((val: {name: string; string: string}) => val.name === props.name);
        if (liveParam) {
            liveParamString = liveParam?.string ? liveParam?.string : '';
        }
        const values = liveParamString ?? props.defaultVal ?? '';
        const index = getValue()?.findIndex((val: {name: string; string: string}) => val.name === props.name) ?? -1;

        return (
            <React.Fragment>
                <ResetOrDeleteButton
                    isPluginPar={props.isPluginPar}
                    getValue={getValue}
                    name={props.name}
                    index={index}
                    setValue={setValue}
                    setAppParamsDeletedState={props.setAppParamsDeletedState}
                />
                <div>
                    <input
                        // disable chrome autocomplete
                        autoComplete='fake'
                        className='argo-field'
                        style={{width: '40%', display: 'inline-block', marginTop: 25}}
                        placeholder='Value'
                        value={values || ''}
                        onChange={e => {
                            if (index >= 0) {
                                getValue()[index].string = e.target.value;
                                setValue([...getValue()]);
                            } else {
                                setValue([...(getValue() || []), {name: props.name, string: e.target.value}]);
                            }
                        }}
                        title='Value'
                    />
                </div>
            </React.Fragment>
        );
    }
);

export const MapInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const items = new Array<NameValue>();
    const map = getValue() || {};
    Object.keys(map).forEach(key => items.push({name: key, value: map[key]}));
    return (
        <ArrayInput
            editor={NameValueEditor}
            items={items}
            onChange={array => {
                const newMap = {} as any;
                array.forEach(item => (newMap[item.name || ''] = item.value || ''));
                setValue(newMap);
            }}
        />
    );
});

export const MapValueField = ReactForm.FormField(
    (props: {fieldApi: ReactForm.FieldApi; name: string; defaultVal: Map<string, string>; isPluginPar: boolean; setAppParamsDeletedState: any}) => {
        const {
            fieldApi: {getValue, setValue}
        } = props;
        const items = new Array<NameValue>();
        const liveParam = getValue()?.find((val: {name: string; map: object}) => val.name === props.name);
        const index = getValue()?.findIndex((val: {name: string; map: object}) => val.name === props.name) ?? -1;
        if (liveParam) {
            liveParam.map = liveParam.map ? liveParam.map : new Map<string, string>();
        }
        if (liveParam?.array) {
            items.push(...liveParam.array);
        } else {
            const map = liveParam?.map ?? props.defaultVal ?? new Map<string, string>();
            Object.keys(map).forEach(item => items.push({name: item || '', value: map[item] || ''}));
            if (liveParam?.map) {
                getValue()[index].array = items;
            }
        }

        return (
            <React.Fragment>
                <ResetOrDeleteButton
                    isPluginPar={props.isPluginPar}
                    getValue={getValue}
                    name={props.name}
                    index={index}
                    setValue={setValue}
                    setAppParamsDeletedState={props.setAppParamsDeletedState}
                />

                <ArrayInput
                    editor={NameValueEditor}
                    items={items || []}
                    onChange={change => {
                        if (index === -1) {
                            getValue().push({
                                name: props.name,
                                array: change
                            });
                        } else {
                            getValue()[index].array = change;
                        }
                        setValue([...getValue()]);
                    }}
                />
            </React.Fragment>
        );
    }
);
