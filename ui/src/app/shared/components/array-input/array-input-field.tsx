import * as React from 'react';
import * as ReactForm from 'react-form';
import {ArrayInput} from './array-input';

class Item {
    public name: string;
    public value: string;
}

const ItemEditor = (i: Item, replaceItem: (i: Item) => void, removeItem: () => void) => (
    <React.Fragment>
        <input value={i.name} onChange={(e) => replaceItem({...i, name: e.target.value})} title='Name'/>
        &nbsp;
        =
        &nbsp;
        <input value={i.value} onChange={(e) => replaceItem({...i, value: e.target.value})} title='Value'/>
        &nbsp;
        <button>
            <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => removeItem()}/>
        </button>
    </React.Fragment>
);

class Props {
    public addItem: (i: Item) => void;
}

export class ItemCreator<I> extends React.Component<Props, Item> {
    constructor(props: Props) {
        super(props);
        this.state = {name: '', value: ''};
    }

    public render() {
        const setName = (name: string) => {
            this.setState((s) => ({...s, name}));
        };
        const setValue = (value: string) => {
            this.setState((s) => ({...s, value}));
        };
        return (
            <div>
                <input placeholder='Name' value={this.state.name} onChange={(e) => setName(e.target.value)}
                       title='Name'/>
                &nbsp;
                =
                &nbsp;
                <input placeholder='Value' value={this.state.value} onChange={(e) => setValue(e.target.value)}
                       title='Value'/>
                &nbsp;
                <input placeholder='Value' value={this.state.value} onChange={(e) => setValue(e.target.value)}/>
                &nbsp;
                <button disabled={this.state.name === '' || this.state.value === ''}
                        onClick={() => {
                            this.props.addItem(this.state);
                            this.setState(() => new Item());
                        }}>
                    <i style={{cursor: 'pointer'}} className='fa fa-plus'/>
                </button>
            </div>
        );
    }
}

export const
    ArrayInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
        const {fieldApi: {getValue, setValue}} = props;
        return (
            <ArrayInput items={getValue() || []} onChange={setValue}
                        itemEditor={ItemEditor}
                        itemCreator={(addItem: (i: Item) => void) => <ItemCreator addItem={addItem}/>}/>
        );
    });

export const
    MapInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
        const {fieldApi: {getValue, setValue}} = props;
        const items = new Array<Item>();
        const map = getValue() || {};
        Object.keys(map).forEach((key) => items.push({name: key, value: map[key]}));
        const onChange = (array: Item[]) => {
            const newMap = {} as any;
            array.forEach((item) => newMap[item.name] = item.value);
            setValue(newMap);
        };
        return (
            <ArrayInput items={items} onChange={onChange} itemEditor={ItemEditor}
                        itemCreator={(addItem: (i: Item) => void) => <ItemCreator addItem={addItem}/>}/>
        );
    });
