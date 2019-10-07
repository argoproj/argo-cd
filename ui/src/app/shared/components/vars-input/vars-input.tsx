import * as React from 'react';
import * as ReactForm from 'react-form';

/*
This is a lift-and-shift of env-input.
 */

class Item {
    public name: string;
    public value: string;
    public code: boolean;
}

class Props {
    public items: Item[];
    public onChange: (items: Item[]) => void;
}

class State {
    public items: Item[];
    public newItem: Item;
}

class VarsInput extends React.Component<Props, State> {
    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {newItem: {name: '', value: '', code: false}, items: props.items};
    }

    public render() {
        const addItem = (i: Item) => {
            this.setState((s) => {
                s.items.push(i);
                this.props.onChange(s.items);
                return {items: s.items, newItem: {name: '', value: '', code: false}};
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
            this.setState((s) => ({items: s.items, newItem: {name, value: s.newItem.value, code: s.newItem.code}}));
        };
        const setValue = (value: string) => {
            this.setState((s) => ({items: s.items, newItem: {name: s.newItem.name, value, code: s.newItem.code}}));
        };
        const setCode = (code: boolean) => {
            this.setState((s) => ({items: s.items, newItem: {name: s.newItem.name, value: s.newItem.value, code}}));
        };
        return (
            <div className='argo-field' style={{border: 0}}>
                <div>
                    {this.state.items.map((i, j) => (
                        <div key={`item-${j}`}>
                            <input value={this.state.items[j].name} title='Name'
                                   onChange={(e) => replaceItem({
                                       name: e.target.value,
                                       value: i.value,
                                       code: i.code,
                                   }, j)}/>
                            &nbsp;
                            =
                            &nbsp;
                            <input value={this.state.items[j].value} title='Value'
                                   onChange={(e) => replaceItem({
                                       name: i.name,
                                       value: e.target.value,
                                       code: i.code,
                                   }, j)}/>
                            &nbsp;
                            <input type='checkbox' checked={this.state.items[j].code} title='Code'
                                   onChange={(e) => replaceItem({
                                       name: i.name,
                                       value: i.value,
                                       code: e.target.checked,
                                   }, j)}/>
                            &nbsp;
                            <button onClick={() => removeItem(j)}>
                                <i className='fa fa-times'/>
                            </button>
                        </div>
                    ))}
                </div>
                <div>
                    <input placeholder='Name' value={this.state.newItem.name} title='Name'
                           onChange={(e) => setName(e.target.value)}/>
                    &nbsp;
                    =
                    &nbsp;
                    <input placeholder='Value' value={this.state.newItem.value} title='Value'
                           onChange={(e) => setValue(e.target.value)}/>
                    &nbsp;
                    <input type='checkbox' checked={this.state.newItem.code} title='Code'
                           onChange={(e) => setCode(e.target.checked)}/>
                    &nbsp;
                    <button disabled={this.state.newItem.name === '' || this.state.newItem.value === ''}
                            onClick={() => addItem(this.state.newItem)}>
                        <i className='fa fa-plus'/>
                    </button>
                </div>
            </div>
        );
    }
}

export const VarsInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    return <VarsInput items={getValue() || []} onChange={setValue}/>;
});
