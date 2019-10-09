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

class Props {
    public addItem: (i: ArrayItem) => void;
}

export class ArrayItemCreator<I> extends React.Component<Props, ArrayItem> {
    constructor(props: Props) {
        super(props);
        this.state = new ArrayItem();
    }

    public render() {
        return (
            <div>
                <input placeholder='Name' value={this.state.name} onChange={(e) =>
                    this.setState((s) => {
                        return {...s, name: e.target.value};
                    })
                }/>
                &nbsp;
                =
                &nbsp;
                <input placeholder='Value' value={this.state.value} onChange={(e) =>
                    this.setState((s) => ({...s, value: e.target.value}))
                }/>
                &nbsp;
                <button disabled={this.state.name === '' || this.state.value === ''}
                        onClick={() => {
                            this.props.addItem(this.state);
                            this.setState(() => new ArrayItem());
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
                        itemEditor={ArrayItemEditor}
                        itemCreator={(addItem: (i: ArrayItem) => void) => <ArrayItemCreator addItem={addItem}/>}/>
        );
    });

export const
    MapInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
        const {fieldApi: {getValue, setValue}} = props;
        const items = new Array<ArrayItem>();
        const map = getValue() || {};
        Object.keys(map).forEach((key) => items.push({name: key, value: map[key]}));
        const onChange = (array: ArrayItem[]) => {
            const newMap = {} as any;
            array.forEach((item) => newMap[item.name] = item.value);
            setValue(newMap);
        };
        return (
            <ArrayInput items={items} onChange={onChange} itemEditor={ArrayItemEditor}
                        itemCreator={(addItem: (i: ArrayItem) => void) => <ArrayItemCreator addItem={addItem}/>}/>
        );
    });

export const
    VarsInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
        const {fieldApi: {getValue, setValue}} = props;
        return (
            <ArrayInput items={getValue() || []} onChange={setValue} itemEditor={ArrayItemEditor}
                        itemCreator={(addItem: (i: ArrayItem) => void) => <ArrayItemCreator addItem={addItem}/>}/>
        );
    });
