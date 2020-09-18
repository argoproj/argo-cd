import * as React from 'react';
import {ResourceKind, ResourceKindSelector} from '../resource-kind-selector';
import {ArgoAutocomplete} from './autocomplete';

export interface FieldData {
    type: FieldTypes;
    name: string;
    size: FieldSizes;
    values?: string[];
}

export enum FieldTypes {
    Text = 'text',
    ResourceKindSelector = 'resourceKindSelector',
    Url = 'url',
    AutoComplete = 'autoComplete'
}

export enum FieldSizes {
    Normal = 'normal',
    Large = 'large',
    Grow = 'grow'
}

export type FieldValue = string | ResourceKind;

export function IsFieldValue<T>(value: T | FieldValue): value is FieldValue {
    if ((typeof value as FieldValue) === 'string') {
        return true;
    }
    return false;
}

export function FieldLabels(fields: FieldData[]): React.ReactFragment {
    return (
        <div className='card__row card__labels card__input-container'>
            {fields.map(field => {
                return (
                    <div className={`card__labels--label card__col-input card__col card__col-${field.size}`} key={field.name + 'label'}>
                        {field.name}
                        {field.type === FieldTypes.ResourceKindSelector ? (
                            <a href='https://kubernetes.io/docs/reference/kubectl/overview/#resource-types' target='_blank' className='card__info-icon'>
                                <i className='fas fa-info-circle' />
                            </a>
                        ) : null}
                    </div>
                );
            })}
        </div>
    );
}

interface ArgoFieldProps {
    field: FieldData;
    onChange: (value: FieldValue) => void;
    data: FieldValue;
}

export class ArgoField extends React.Component<ArgoFieldProps> {
    public render() {
        let format;
        const field = this.props.field;
        switch (field.type) {
            case FieldTypes.ResourceKindSelector:
                format = <ResourceKindSelector placeholder={field.name} init={this.props.data as ResourceKind} onChange={this.props.onChange} />;
                break;
            case FieldTypes.AutoComplete:
                format = <ArgoAutocomplete values={field.values || []} placeholder={field.name} onChange={this.props.onChange} init={this.props.data as FieldValue} />;
                break;
            default:
                format = (
                    <input
                        type='text'
                        className='card--input'
                        value={this.props.data ? this.props.data.toString() : ''}
                        onChange={e => this.props.onChange(e.target.value)}
                        placeholder={field.name}
                    />
                );
        }
        return (
            <div style={{width: '100%'}}>
                {format}
                {field.type === FieldTypes.Url && (this.props.data as string) !== '' && (this.props.data as string) !== null && (this.props.data as string) !== '*' ? (
                    <a className='card__link-icon' href={this.props.data as string} target='_blank'>
                        <i className='fas fa-link' />
                    </a>
                ) : null}
            </div>
        );
    }
}
