import * as React from 'react';
import {ResourceKind, ResourceKindSelector} from '../../../../shared/components/project/resource-kind-selector';
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

interface ArgoFieldProps {
    field: FieldData;
    onChange: (value: FieldValue) => void;
    data: FieldValue;
    index: number;
}

export class ArgoField extends React.Component<ArgoFieldProps> {
    public render() {
        const field = this.props.field;
        let format;
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
                        className='argo-field'
                        value={this.props.data ? this.props.data.toString() : ''}
                        onChange={e => this.props.onChange(e.target.value)}
                        placeholder={field.name}
                    />
                );
        }
        return (
            <div style={{width: '100%', display: 'flex'}}>
                {format}
                {field.type === FieldTypes.Url && (this.props.data as string) !== '' && (this.props.data as string) !== null && (this.props.data as string) !== '*' ? (
                    <a href={this.props.data as string} target='_blank'>
                        <i className='fas fa-link' />
                    </a>
                ) : null}
            </div>
        );
    }
}
