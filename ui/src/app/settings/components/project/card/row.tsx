import * as React from 'react';
import {Project} from '../../../../shared/models';
import {GetProp, SetProp} from '../../utils';
import {ResourceKind, ResourceKindSelector} from '../resource-kind-selector';

// two options here:
// *support non-object types or strings as fields in CardRow,* (leaning toward this)
// OR
// convert structured object data to and from string arrays in ProjectSummary

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
    data: T | FieldValue;
    remove: () => void;
    save: (value: T | FieldValue) => Promise<Project>;
}

interface CardRowState<T> {
    data: T | FieldValue;
    error: boolean;
}

export type FieldValue = string | ResourceKind;

export class CardRow<T> extends React.Component<CardRowProps<T>, CardRowState<T>> {
    get changed(): boolean {
        if (this.dataIsFieldValue) {
            return this.state.data !== this.props.data;
        }
        for (const key of Object.keys(this.props.data)) {
            if (GetProp(this.props.data as T, key as keyof T) !== GetProp(this.state.data as T, key as keyof T)) {
                return true;
            }
        }
        return false;
    }
    get dataIsFieldValue(): boolean {
        return this.isFieldValue(this.state.data);
    }
    constructor(props: CardRowProps<T>) {
        super(props);
        this.state = {
            data: this.props.data,
            error: false
        };
        this.save = this.save.bind(this);
    }

    public render() {
        let update = this.dataIsFieldValue
            ? (value: FieldValue, _: keyof T) => {
                  this.setState({data: value as FieldValue});
              }
            : (value: string, field: keyof T) => {
                  const change = {...(this.state.data as T)};
                  SetProp(change, field, value);
                  this.setState({data: change});
              };
        update = update.bind(this);
        const inputs = this.props.fields.map((field, i) => {
            let format;
            switch (field.type) {
                case FieldTypes.ResourceKindSelector:
                    format = <ResourceKindSelector />;
                    break;
                default:
                    const curVal = this.dataIsFieldValue ? this.state.data : GetProp(this.state.data as T, field.name as keyof T);
                    format = <input type='text' value={curVal ? curVal.toString() : ''} onChange={e => update(e.target.value, field.name as keyof T)} placeholder={field.name} />;
            }
            return <div key={field.name + '.' + i}>{format}</div>;
        });

        return (
            <div className='card__input-container card__row'>
                <div className='card__col-round-button card__col'>
                    <button className='project__button project__button-remove project__button-round' onClick={this.props.remove}>
                        -
                    </button>
                </div>
                <div className='card__col-input card__col'>{inputs}</div>
                <div className='card__col-button card__col'>
                    <button className={`project__button project__button-${this.state.error ? 'error' : this.changed ? 'save' : 'saved'}`} onClick={() => this.save()}>
                        {this.state.error ? 'ERROR' : this.changed ? 'SAVE' : 'SAVED'}
                    </button>
                </div>
            </div>
        );
    }
    private isFieldValue(value: T | FieldValue): value is FieldValue {
        if ((typeof value as FieldValue) === 'string') {
            return true;
        }
        return false;
    }
    private async save() {
        const err = await this.props.save(this.state.data);
        console.log(err);
        // this.setState({error: err});
    }
}
