import * as React from 'react';
import * as ReactAutocomplete from 'react-autocomplete';

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
}

export class ArgoField extends React.Component<ArgoFieldProps> {
    public render() {
        let format;
        switch (this.props.field.type) {
            case FieldTypes.AutoComplete:
                format = (
                    <ReactAutocomplete
                        wrapperStyle={{display: 'block', width: '100%'}}
                        items={this.props.field.values || []}
                        onSelect={(_, item: string) => {
                            this.props.onChange(item);
                        }}
                        getItemValue={item => item}
                        value={this.props.data ? this.props.data.toString() : ''}
                        onChange={e => this.props.onChange(e.target.value)}
                        shouldItemRender={(item: string, val: string) => {
                            return item.toLowerCase().indexOf(val.toLowerCase()) > -1;
                        }}
                        renderItem={(item, isSelected) => (
                            <div className={`select__option ${isSelected ? 'selected' : ''}`} key={item}>
                                {item}
                            </div>
                        )}
                        renderMenu={function(menuItems, _, style) {
                            if (menuItems.length === 0) {
                                return <div style={{display: 'none'}} />;
                            }
                            return <div style={{...style, ...this.menuStyle, display: 'block', color: 'white', zIndex: 10, maxHeight: '20em'}} children={menuItems} />;
                        }}
                        renderInput={inputProps => <input {...inputProps} className='argo-field' placeholder={this.props.field.name} />}
                    />
                );
                break;
            default:
                format = (
                    <input
                        type='text'
                        className='argo-field'
                        value={this.props.data ? this.props.data.toString() : ''}
                        onChange={e => this.props.onChange(e.target.value)}
                        placeholder={this.props.field.name}
                    />
                );
        }
        return (
            <div style={{width: '100%', display: 'flex'}}>
                {format}
                {this.props.field.type === FieldTypes.Url && this.props.data !== '' && this.props.data !== null && this.props.data !== '*' ? (
                    <a href={this.props.data as string} target='_blank'>
                        <i className='fas fa-link' />
                    </a>
                ) : null}
            </div>
        );
    }
}
