import * as React from 'react';
import * as ReactForm from 'react-form';
import {ArrayInput} from './array-input';

class Item {
    public name: string;
    public value: string;
    public code: boolean;
}

const ItemEditor = (i: Item, replaceItem: (i: Item) => void, removeItem: () => void) => (
    <React.Fragment>
        <input value={i.name} onChange={(e) => replaceItem({...i, name: e.target.value})}/>
        &nbsp;
        =
        &nbsp;
        <input value={i.value} onChange={(e) => replaceItem({...i, value: e.target.value})}/>
        &nbsp;
        <input checked={i.code} onChange={(e) => replaceItem({...i, code: e.target.checked})}/>
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
        this.state = {name: '', value: '', code: false};
    }

    public render() {
        const setName = (name: string) => {
            this.setState((s) => ({...s, name}));
        };
        const setValue = (value: string) => {
            this.setState((s) => ({...s, value}));
        };
        const setCode = (code: boolean) => {
            this.setState((s) => ({...s, code}));
        };
        return (
            <div>
                <input placeholder='Name' value={this.state.name} onChange={(e) => setName(e.target.value)}/>
                &nbsp;
                =
                &nbsp;
                <input placeholder='Value' value={this.state.value} onChange={(e) => setValue(e.target.value)}/>
                &nbsp;
                <input checked={this.state.code} onChange={(e) => setCode(e.target.checked)}/>
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
    VarsInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
        const {fieldApi: {getValue, setValue}} = props;
        return (
            <ArrayInput items={getValue() || []} onChange={setValue} itemEditor={ItemEditor}
                        itemCreator={(addItem: (i: Item) => void) => <ItemCreator addItem={addItem}/>}/>
        );
    });
