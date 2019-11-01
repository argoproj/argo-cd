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

export const NameValueEditor = (item: NameValue, onChange: (item: NameValue) => void) => (
    <React.Fragment>
        <input placeholder='Name' value={item.name || ''} onChange={e => onChange({...item, name: e.target.value})} title='Name' />
        &nbsp; = &nbsp;
        <input placeholder='Value' value={item.value || ''} onChange={e => onChange({...item, value: e.target.value})} title='Value' />
        &nbsp;
    </React.Fragment>
);

export const ValueEditor = (item: string, onChange: (item: string) => void) => (
    <React.Fragment>
        <input placeholder='Value' value={item || ''} onChange={e => onChange(e.target.value)} title='Value' />
    </React.Fragment>
);

interface Props<T> {
    templateItem: T;
    items: T[];
    onChange: (items: T[]) => void;
    editor: (item: T, onChange: (updated: T) => any) => React.ReactNode;
    valid: (item: T) => boolean;
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
    const [newItem, setNewItem] = React.useState(props.templateItem);

    return (
        <div className='argo-field' style={{border: 0}}>
            <div>
                {props.items.map((item, i) => (
                    <div key={`item-${i}`}>
                        {props.editor(item, (updated: T) => replaceItem(updated, i))}
                        &nbsp;
                        <button>
                            <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => removeItem(i)} />
                        </button>
                    </div>
                ))}
                <div>
                    {props.editor(newItem, setNewItem)}
                    &nbsp;
                    <button
                        disabled={!props.valid(newItem)}
                        onClick={() => {
                            addItem(newItem);
                            setNewItem(props.templateItem);
                        }}>
                        <i style={{cursor: 'pointer'}} className='fa fa-plus' />
                    </button>
                </div>
            </div>
        </div>
    );
}

export function hasNameAndValue(item: {name?: string; value?: string}) {
    return (item.name || '').trim() !== '' && (item.value || '').trim() !== '';
}

export const ArrayInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    return <ArrayInput editor={NameValueEditor} items={getValue() || []} onChange={setValue} valid={hasNameAndValue} templateItem={{}} />;
});

export const ValueArrayInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    return <ArrayInput editor={ValueEditor} items={getValue() || []} onChange={setValue} valid={item => item !== ''} templateItem='' />;
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
            valid={hasNameAndValue}
            onChange={array => {
                const newMap = {} as any;
                array.forEach(item => (newMap[item.name] = item.value));
                setValue(newMap);
            }}
            templateItem={{name: '', value: ''}}
        />
    );
});
