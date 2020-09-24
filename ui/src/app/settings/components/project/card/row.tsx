import * as React from 'react';
import {GetProp, SetProp} from '../../utils';
import {ArgoField, FieldData} from './field';

interface CardRowProps<T> {
    fields: FieldData[];
    data: T | string;
    remove: () => void;
    selected: boolean;
    toggleSelect: () => void;
    changed: boolean;
    onChange: (value: T | string) => void;
    index?: number;
}

export function FieldLabels(fields: FieldData[], edit: boolean): React.ReactFragment {
    return (
        <div className='card__row'>
            {edit && <div className='card__col-select-button card__col' />}
            {fields.map(field => (
                <div className={`card__col-input card__col card__col-${field.size} card__label`} key={field.name + 'label'}>
                    <b>{field.name}</b>
                </div>
            ))}
        </div>
    );
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
                        <button className={`card__button card__button-select${this.props.selected ? '--selected' : ''}`} onClick={this.props.toggleSelect}>
                            <i className='fa fa-check' />
                        </button>
                    </div>
                    {this.props.fields.map((field, i) => {
                        let curVal = '';
                        if (this.props.data) {
                            if (this.isString(this.props.data)) {
                                curVal = this.props.data;
                            } else {
                                const data = GetProp(this.props.data as T, field.name as keyof T);
                                curVal = data ? data.toString() : '';
                            }
                        }
                        return (
                            <div key={field.name} className={`card__col-input card__col card__col-${field.size}`}>
                                <ArgoField field={field} onChange={val => update(val, field.name as keyof T)} data={curVal} index={this.props.index || 0} />
                            </div>
                        );
                    })}
                    {this.fieldsSetToAll.length > 0 ? <i className='fa fa-info-circle' /> : null}
                    {this.props.selected ? (
                        <div className='card__col-button card__col'>
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
