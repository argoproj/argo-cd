import {Checkbox} from 'argo-ui';
import * as React from 'react';
import {GetProp, SetProp} from '../../../settings/components/utils';
import {ArgoField, FieldData} from './field';

interface CardRowProps<T> {
    fields: FieldData[];
    data: T | string;
    remove: () => void;
    selected: boolean;
    toggleSelect: () => void;
    onChange: (value: T | string) => void;
}

export class CardRow<T> extends React.Component<CardRowProps<T>> {
    get disabled(): boolean {
        if (!this.props.data) {
            return true;
        } else if (Object.keys(this.props.data).length < this.props.fields.length) {
            return true;
        }
        for (const key of Object.keys(this.props.data)) {
            const data = GetProp(this.props.data as T, key as keyof T);
            if (data === null) {
                return true;
            } else if (data.toString() === '') {
                return true;
            }
        }
        return false;
    }
    get fieldsSetToAll(): string[] {
        if (!this.props.data) {
            return [];
        }
        if (this.isString(this.props.data)) {
            const field = this.props.fields[0];
            return this.props.data.toString() === '*' ? [field.name] : [];
        }
        const fields = [];
        for (const key of Object.keys(this.props.data)) {
            if (GetProp(this.props.data as T, key as keyof T).toString() === '*') {
                fields.push(key);
            }
        }
        return fields;
    }
    constructor(props: CardRowProps<T>) {
        super(props);
        this.state = {
            data: this.props.data
        };
    }

    public render() {
        let update = this.isString(this.props.data)
            ? (value: string, _: keyof T) => {
                  this.props.onChange(value);
              }
            : (value: string, field: keyof T) => {
                  const change = {...(this.props.data as T)};
                  SetProp(change, field, value);
                  this.props.onChange(change);
              };
        update = update.bind(this);
        return (
            <React.Fragment>
                <div className='card__row'>
                    <div className='card__col-select-button card__col'>
                        <Checkbox onChange={this.props.toggleSelect} checked={this.props.selected} />
                    </div>
                    {this.props.fields.map((field, i) => {
                        const d = this.props.data || '';
                        return (
                            <div key={field.name} className={`card__col card__col-${field.size}`}>
                                <ArgoField
                                    field={field}
                                    onChange={val => update(val, field.name as keyof T)}
                                    data={this.isString(d) ? d : (GetProp(d as T, field.name as keyof T) || '').toString()}
                                />
                            </div>
                        );
                    })}
                    {this.props.selected ? (
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => (this.props.selected ? this.props.remove() : null)}>
                                DELETE
                            </button>
                        </div>
                    ) : null}
                </div>
            </React.Fragment>
        );
    }
    private isString(x: any): x is string {
        return typeof x === 'string';
    }
}
