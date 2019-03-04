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
    value?: string;
    inputProps?: React.HTMLProps<HTMLInputElement>;
    wrapperProps?: React.HTMLProps<HTMLDivElement>;
    renderInput?: (props: React.HTMLProps<HTMLInputElement>) => React.ReactNode;
    renderItem?: (item: AutocompleteOption) => React.ReactNode;
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
    const [autocompleteEl, setAutocompleteEl] = React.useState(null);

    React.useEffect(() => {
        const listener = () => {
            if (autocompleteEl && autocompleteEl.refs.input) {
                autocompleteEl.setMenuPositions();
            }
        };
        document.addEventListener('scroll', listener, true);
        return () => {
            document.removeEventListener('scroll', listener);
        };
    });

    const wrapperProps = props.wrapperProps || {};
    wrapperProps.className = classNames('select', wrapperProps.className);
    return (
        <ReactAutocomplete
            ref={(el: any) => {
                setAutocompleteEl(el);
                if (el) {
                    // workaround for 'autofill for forms not deactivatable' https://bugs.chromium.org/p/chromium/issues/detail?id=370363#c7
                    (el.refs.input as HTMLInputElement).autocomplete = 'no-autocomplete';
                }
                if (props.autoCompleteRef) {
                    props.autoCompleteRef({ refresh: () => {
                        if (el && el.refs.input) {
                            el.setMenuPositions();
                        }
                    } });
                }
            }}
            inputProps={props.inputProps}
            wrapperProps={wrapperProps}
            shouldItemRender={(item: AutocompleteOption, val: string) => {
                return item.label.includes(val);
            }}
            renderMenu={function(menuItems, _, style) {
                if (menuItems.length === 0) {
                    return <div style={{ display: 'none' }}/>;
                }
                return <div style={{ ...style, ...this.menuStyle, background: 'white', zIndex: 10, maxHeight: '20em' }} children={menuItems} />;
            }}
            getItemValue={(item) => item.label}
            items={items}
            value={props.value}
            renderItem={(item, isSelected) => (
                <div className={classNames('select__option', { selected: isSelected })} key={item.label}>
                    {props.renderItem && props.renderItem(item) || item.label}
                </div>
            )}
            onChange={props.onChange}
            onSelect={props.onSelect}
            renderInput={props.renderInput}
        />
    );
};
