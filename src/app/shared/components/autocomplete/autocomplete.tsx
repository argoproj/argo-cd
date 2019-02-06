import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactAutocomplete from 'react-autocomplete';

export interface AutocompleteApi {
    refresh(): any;
}

export interface AutocompleteOption {
    value: string;
    label?: string;
}

export interface AutocompleteProps {
    items: (AutocompleteOption | string)[];
    input: string;
    inputProps?: React.HTMLProps<HTMLInputElement>;
    renderInput?: (props: React.HTMLProps<HTMLInputElement>) => React.ReactNode;
    onChange?: (e: React.ChangeEvent<HTMLInputElement>, value: string) => void;
    onSelect?: (value: string, item: any) => void;
    autoCompleteRef?: (api: AutocompleteApi) => any;
}

export const Autocomplete = (props: AutocompleteProps) => {
    const items = (props.items || []).map((item) => {
        if (typeof item === 'string') {
            return { value: item, label: item };
        } else {
            return {
                value: item.value,
                label: item.label || item.value,
            };
        }
    });
    return (
        <ReactAutocomplete
            ref={(el: any) => {
                if (props.autoCompleteRef) {
                    props.autoCompleteRef({ refresh: () => el.setMenuPositions() });
                }
            }}
            inputProps={props.inputProps}
            wrapperProps={{ className: 'select' }}
            shouldItemRender={(item: AutocompleteOption, val: string) => {
                return item.label.includes(val);
            }}
            renderMenu={function(menuItems, _, style) {
                if (menuItems.length === 0) {
                    return <div style={{ display: 'none' }}/>;
                }
                return <div style={{ ...style, ...this.menuStyle, background: 'white', zIndex: 10, maxHeight: '200px' }} children={menuItems} />;
            }}
            getItemValue={(item) => item.label}
            items={items}
            value={props.input}
            renderItem={(item, isSelected) => (
                <div className={classNames('select__option', { selected: isSelected })} key={item.label}>{item.label}</div>
            )}
            onChange={props.onChange}
            onSelect={props.onSelect}
            renderInput={props.renderInput}
        />
    );
};
