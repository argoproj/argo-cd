import * as React from 'react';
import * as ReactForm from 'react-form';

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

export const NameValueEditor = (item: NameValue, onChange: (item: NameValue) => any) => (
    <React.Fragment>
        <input
            // disable chrome autocomplete
            autoComplete='fake'
            className='argo-field'
            style={{width: '40%'}}
            placeholder='Name'
            value={item.name || ''}
            onChange={e => onChange({...item, name: e.target.value})}
            title='Name'
        />
        &nbsp; = &nbsp;
        <input
            // disable chrome autocomplete
            autoComplete='fake'
            className='argo-field'
            style={{width: '40%'}}
            placeholder='Value'
            value={item.value || ''}
            onChange={e => onChange({...item, value: e.target.value})}
            title='Value'
        />
        &nbsp;
    </React.Fragment>
);

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

export const ArrayInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    return <ArrayInput editor={NameValueEditor} items={getValue() || []} onChange={setValue} />;
});

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
