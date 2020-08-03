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

function getProp<T, K extends keyof T>(obj: T, key: K) {
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

    get changed(): boolean {
        for (const key of Object.keys(this.props.data)) {
            if (getProp(this.props.data, key as keyof T) !== getProp(this.state.data, key as keyof T)) {
                return true;
            }
        }
        return false;
    }

    public render() {
        const inputs = this.props.fields.map(field => {
            let format;
            switch (field.type) {
                case FieldTypes.ResourceKindSelector:
                    format = <ResourceKindSelector />;
                    break;
                default:
                    const curVal = getProp(this.state.data, field.name as keyof T);
                    format = (
                        <input
                            type='text'
                            value={curVal ? curVal.toString() : ''}
                            onChange={e => {
                                const change = {...this.state.data};
                                setProp(change, field.name as keyof T, e.target.value);
                                this.setState({data: change});
                            }}
                            placeholder={field.name}
                        />
                    );
            }
            return format;
        });

        return (
            <div className='card__input-container card__row'>
                <div className='card__col-round-button card__col'>
                    <button className='project__button project__button-remove project__button-round'>-</button>
                </div>
                <div className='card__col-input card__col'>
                    {inputs}
                </div>
                <div className='card__col-button card__col'>
                    <button className={`project__button project__button-${this.changed ? 'save' : 'saved'}`}>{this.changed ? 'SAVE' : 'SAVED'}</button>
                </div>
            </div>
        )
    };
}
