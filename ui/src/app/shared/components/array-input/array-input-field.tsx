import * as React from 'react';
import * as ReactForm from 'react-form';
import {ArrayInput} from './array-input';

class ArrayItem {
    public name: string;
    public value: string;
}

const ArrayItemEditor = (i: ArrayItem, replaceItem: (i: ArrayItem) => void, removeItem: () => void) => (
    <React.Fragment>
        <input value={i.name} onChange={(e) => replaceItem({...i, name: e.target.value})}/>
        &nbsp;
        =
        &nbsp;
        <input value={i.value} onChange={(e) => replaceItem({...i, value: e.target.value})}/>
        &nbsp;
        <button>
            <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => removeItem()}/>
        </button>
    </React.Fragment>
);

const ArrayItemCreator = (i: ArrayItem, addItem: () => void) => (
    <div>
        <input placeholder='Name' value={i.name} onChange={(e) => {
            i.name = e.target.value;
        }}/>
        &nbsp;
        =
        &nbsp;
        <input placeholder='Value' value={i.value} onChange={(e) => {
            i.value = e.target.value;
        }}/>
        &nbsp;
        <button disabled={i.name === '' || i.value === ''} onClick={() => addItem()}>
            <i style={{cursor: 'pointer'}} className='fa fa-plus'/>
        </button>
    </div>

);

export const ArrayInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    return (
        <ArrayInput items={getValue() || []} onChange={setValue} emptyItem={() => new ArrayItem()}
                    itemEditor={ArrayItemEditor} itemCreator={ArrayItemCreator}/>
    );
});

export const MapInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    const items = new Array<ArrayItem>();
    const map = getValue() || {};
    Object.keys(map).forEach((key) => items.push({name: key, value: map[key]}));
    return (
        <ArrayInput items={items} onChange={(array) => {
            const newMap = {} as any;
            array.forEach((item) => newMap[item.name] = item.value);
            setValue(newMap);
        }} emptyItem={() => new ArrayItem()} itemEditor={ArrayItemEditor} itemCreator={ArrayItemCreator}/>
    );
});

export const VarsInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    return (
        <ArrayInput items={getValue() || []} onChange={setValue} emptyItem={() => new ArrayItem()}
                    itemEditor={ArrayItemEditor} itemCreator={ArrayItemCreator}/>
    );
});
