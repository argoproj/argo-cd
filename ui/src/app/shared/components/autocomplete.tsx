import * as React from 'react';
import * as ReactAutocomplete from 'react-autocomplete';

interface AutocompleteProps {
    onChange: (value: string) => void;
    init: string;
    values: string[];
    placeholder: string;
}

export class ArgoAutocomplete extends React.Component<AutocompleteProps> {
    public render() {
        return (
            <ReactAutocomplete
                wrapperStyle={{display: 'block', width: '100%'}}
                items={this.props.values}
                onSelect={(_, item: string) => {
                    this.props.onChange(item);
                }}
                getItemValue={item => item}
                value={this.props.init ? this.props.init.toString() : ''}
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
                renderInput={inputProps => <input {...inputProps} className='argo-field' placeholder={this.props.placeholder} />}
            />
        );
    }
}
