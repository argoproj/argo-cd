import * as React from 'react';
import {ArgoAutocomplete} from '../../../../shared/components/autocomplete';

export interface FieldData {
    type: FieldTypes;
    name: string;
    size: FieldSizes;
    values?: string[];
}

export enum FieldTypes {
    Text = 'text',
    Url = 'url',
    AutoComplete = 'autoComplete'
}

export enum FieldSizes {
    Normal = 'normal',
    Large = 'large',
    Grow = 'grow'
}

interface ArgoFieldProps {
    field: FieldData;
    onChange: (value: string) => void;
    data: string;
    index: number;
}

export class ArgoField extends React.Component<ArgoFieldProps> {
    public render() {
        const field = this.props.field;
        let format;
        switch (field.type) {
            case FieldTypes.AutoComplete:
                format = <ArgoAutocomplete values={field.values || []} placeholder={field.name} onChange={this.props.onChange} init={this.props.data} />;
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
                {field.type === FieldTypes.Url && this.props.data !== '' && this.props.data !== null && this.props.data !== '*' ? (
                    <a href={this.props.data as string} target='_blank'>
                        <i className='fas fa-link' />
                    </a>
                ) : null}
            </div>
        );
    }
}
