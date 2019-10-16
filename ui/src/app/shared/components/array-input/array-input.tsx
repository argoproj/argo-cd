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

class Item {
    public name: string;
    public value: string;
}

class Props {
    public items: Item[];
    public onChange: (items: Item[]) => void;
}

class State {
    public items: Item[];
    public newItem: Item;
}

export class ArrayInput extends React.Component<Props, State> {
    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {newItem: {name: '', value: ''}, items: props.items};
    }

    public render() {
        const addItem = (i: Item) => {
            this.setState((s) => {
                s.items.push(i);
                this.props.onChange(s.items);
                return {items: s.items, newItem: {name: '', value: ''}};
            });
        };
        const replaceItem = (i: Item, j: number) => {
            this.setState((s) => {
                s.items[j] = i;
                this.props.onChange(s.items);
                return s;
            });
        };
        const removeItem = (j: number) => {
            this.setState((s) => {
                s.items.splice(j, 1);
                this.props.onChange(s.items);
                return s;
            });
        };
        const setName = (name: string) => {
            this.setState((s) => ({items: s.items, newItem: {name, value: s.newItem.value}}));
        };
        const setValue = (value: string) => {
            this.setState((s) => ({items: s.items, newItem: {name: s.newItem.name, value}}));
        };
        return (
            <div className='argo-field' style={{border: 0}}>
                <div>
                    {this.state.items.map((i, j) => (
                        <div key={`item-${j}`}>
                            <input value={this.state.items[j].name}
                                   onChange={(e) => replaceItem({name: e.target.value, value: i.value}, j)}/>
                            &nbsp;
                            =
                            &nbsp;
                            <input value={this.state.items[j].value}
                                   onChange={(e) => replaceItem({name: i.name, value: e.target.value}, j)}/>
                            &nbsp;
                            <button >
                                <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => removeItem(j)}/>
                            </button>
                        </div>
                    ))}
                </div>
                <div>
                    <input placeholder='Name' value={this.state.newItem.name}
                           onChange={(e) => setName(e.target.value)}/>
                    &nbsp;
                    =
                    &nbsp;
                    <input placeholder='Value' value={this.state.newItem.value}
                           onChange={(e) => setValue(e.target.value)}/>
                    &nbsp;
                    <button disabled={this.state.newItem.name === '' || this.state.newItem.value === ''}
                            onClick={() => addItem(this.state.newItem)}>
                        <i style={{cursor: 'pointer'}} className='fa fa-plus'/>
                    </button>
                </div>
            </div>
        );
    }
}

export const ArrayInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    return <ArrayInput items={getValue() || []} onChange={setValue}/>;
});

export const MapInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    const items = new Array<Item>();
    const map = getValue() || {};
    Object.keys(map).forEach((key) => items.push({ name: key, value: map[key] }));
    return (
        <ArrayInput items={items} onChange={(array) => {
            const newMap = {} as any;
            array.forEach((item) => newMap[item.name] = item.value);
            setValue(newMap);
        }}/>
    );
});
