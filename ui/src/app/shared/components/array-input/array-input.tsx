import * as React from 'react';
import * as ReactForm from 'react-form';

class Item {
    public name: string;
    public value: string;
}

class Props {
    public setValue: (value: Item[]) => void;
    public getValue: () => Item[];
}

class State {
    public name: string;
    public value: string;
}

class ArrayInput extends React.Component<Props, State> {
    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {name: '', value: ''};
    }

    public render() {
        // replace the existing value
        const replaceValue = (name: string, value: string) => (e: any) => {
            this.props.setValue((this.props.getValue() || []).map((i) => ({
                name: i.name,
                value: i.name === name && i.value === value ? e.target.value : i.value,
            })));
        };
        const removeItem = (name: string, value: string) => () => {
            this.props.setValue((this.props.getValue() || []).filter((i) => i.name !== name || i.value !== value));
        };
        const addItem = () => {
            const prevValue = this.props.getValue() || [];
            prevValue.push({name: this.state.name, value: this.state.value});
            this.props.setValue(prevValue);
            this.setState({name: '', value: ''});
        };

        const setName = (name: string) => {
            this.setState((s) => ({name, value: s && s.value}));
        };
        const setValue = (value: string) => {
            this.setState((s) => ({name: s && s.name, value}));
        };

        return (
            <div className='argo-field' style={{border: 0}}>
                <React.Fragment key='existing'>
                    {(this.props.getValue() || []).map((i) => (
                        <div key={`item-${i.name}`}>
                            <input value={i.name} disabled={true}/>
                            =
                            <input value={i.value} onChange={replaceValue(i.name, i.value)}/>

                            <button onClick={removeItem(i.name, i.value)}>
                                <i className='fa fa-times'/>
                            </button>
                        </div>
                    ))}
                </React.Fragment>
                <div>
                    <input placeholder='Name' value={this.state.name} onChange={(e) => setName(e.target.value)}/>
                    =
                    <input placeholder='Value' value={this.state.value} onChange={(e) => setValue(e.target.value)}/>

                    <button disabled={this.state.name === '' || this.state.value === ''} onClick={() => addItem()}>
                        <i className='fa fa-plus'/>
                    </button>
                </div>
            </div>
        );
    }
}

export const ArrayInputField = ReactForm.FormField((props: { fieldApi: ReactForm.FieldApi }) => {
    const {fieldApi: {getValue, setValue}} = props;
    return <ArrayInput getValue={getValue} setValue={setValue}/>;
});
