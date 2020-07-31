import * as React from 'react';
import {ResourceKind, ResourceKindSelector} from '../resource-kind-selector';

export interface FieldData {
    type: FieldTypes;
    name: string;
}

export enum FieldTypes {
    Text = 'text',
    ResourceKindSelector = 'resourceKindSelector'
}

interface CardRowProps<T> {
    fields: FieldData[];
    data: T;
}

interface CardRowState<T> {
    data: T;
}

function prop<T, K extends keyof T>(obj: T, key: K) {
    return obj[key];
}

function setProp<T, K extends keyof T, S>(obj: T, key: K, value: any) {
    obj[key] = value;
}

export type FieldValue = string | ResourceKind;

export class CardRow<T> extends React.Component<CardRowProps<T>, CardRowState<T>> {
    constructor(props: CardRowProps<T>) {
        super(props);
        this.state = {
            data: this.props.data
        };
    }

    public render() {
        const row = this.props.fields.map(field => {
            let format;
            switch (field.type) {
                case FieldTypes.ResourceKindSelector:
                    format = <ResourceKindSelector />;
                    break;
                default:
                    format = (
                        <input
                            type='text'
                            value={prop(this.state.data, field.name as keyof T).toString()}
                            onChange={e => {
                                const change = {...this.state.data};
                                setProp(change, field.name as keyof T, e.target.value);
                                this.setState({data: change});
                            }}
                            placeholder={field.name}
                        />
                    );
            }
            return (
                <div className='card__col-input card__col' key={field.type + '.' + field.name}>
                    {format}
                </div>
            );
        });

        return <div>{row}</div>
    };
}
