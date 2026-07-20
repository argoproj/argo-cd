import * as React from 'react';
import {Autocomplete} from 'argo-ui';
import classNames from 'classnames';
import {Key, KeybindingContext} from 'argo-ui/v2';

import {isInvalidRegex} from '../../utils';
import './search-bar.scss';

interface SearchBarProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    /** Disable keyboard shortcuts (useful when parent has custom handling) */
    disableKeyboardShortcuts?: boolean;
    /** When true, the input's border switches to a teal/red active/invalid state based on
     *  whether `value` parses as a valid RegExp. The toggle button lives outside the SearchBar. */
    regexEnabled?: boolean;
    /** Optional autocomplete configuration */
    autocomplete?: {
        items: Array<string | {value: string; label: string}>;
        onSelect: (value: string) => void;
        renderItem?: (item: {value: string; label: string}) => React.ReactNode;
        filterSuggestions?: boolean;
    };
}

export const SearchBar: React.FC<SearchBarProps> = ({value, onChange, placeholder = 'Search...', disableKeyboardShortcuts = false, regexEnabled, autocomplete}) => {
    const inputRef = React.useRef<HTMLInputElement>(null);
    const searchBarRef = React.useRef<HTMLDivElement>(null);
    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);
    const [localValue, setLocalValue] = React.useState(value);

    // Sync local value with prop value when it changes externally
    React.useEffect(() => {
        setLocalValue(value);
    }, [value]);

    const handleChange = (newValue: string) => {
        setLocalValue(newValue);
        onChange(newValue);
    };

    const regexInvalid = regexEnabled && isInvalidRegex(value);

    const inputClassName = classNames('search-bar__input', {
        'search-bar__input--regex': regexEnabled && !regexInvalid,
        'search-bar__input--regex-invalid': regexInvalid
    });

    const focusInput = () => {
        if (autocomplete && searchBarRef.current) {
            searchBarRef.current.querySelector('input')?.focus();
        } else {
            inputRef.current?.focus();
        }
    };

    const blurInput = () => {
        if (autocomplete && searchBarRef.current) {
            searchBarRef.current.querySelector('input')?.blur();
        } else {
            inputRef.current?.blur();
        }
        setFocus(false);
    };

    // Register global slash keybinding to focus search
    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (disableKeyboardShortcuts || isFocused) {
                return false;
            }
            focusInput();
            return true;
        }
    });

    // Register global escape keybinding to blur search
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (disableKeyboardShortcuts || !isFocused) {
                return false;
            }
            blurInput();
            return true;
        }
    });

    // If autocomplete is provided, use Autocomplete component
    if (autocomplete) {
        // Normalize items to {value, label} format
        const normalizedItems = autocomplete.items.map(item => (typeof item === 'string' ? {value: item, label: item} : item));

        // In regex mode, Autocomplete's built-in substring filter would hide valid regex matches,
        // so we pre-filter with the pattern ourselves and disable its filter.
        let effectiveItems = normalizedItems;
        let effectiveFilter = autocomplete.filterSuggestions ?? true;
        if (regexEnabled) {
            effectiveFilter = false;
            if (value) {
                if (regexInvalid) {
                    effectiveItems = [];
                } else {
                    const re = new RegExp(value);
                    effectiveItems = normalizedItems.filter(item => re.test(item.value));
                }
            }
        }

        return (
            <Autocomplete
                filterSuggestions={effectiveFilter}
                renderInput={inputProps => (
                    <div className={inputClassName} ref={searchBarRef}>
                        <i className='fa fa-search' style={{marginRight: '9px', cursor: 'pointer'}} onClick={focusInput} />
                        <input
                            {...inputProps}
                            onFocus={e => {
                                setFocus(true);
                                e.target.select();
                                if (inputProps.onFocus) {
                                    inputProps.onFocus(e);
                                }
                            }}
                            onBlur={e => {
                                setFocus(false);
                                if (inputProps.onBlur) {
                                    inputProps.onBlur(e);
                                }
                            }}
                            style={{fontSize: '14px', flex: 1, minWidth: 0}}
                            className='argo-field'
                            placeholder={placeholder}
                        />
                        <div className='keyboard-hint'>/</div>
                        {value && <i className='fa fa-times' onClick={() => handleChange('')} style={{cursor: 'pointer', marginLeft: '5px'}} />}
                    </div>
                )}
                wrapperProps={{className: 'search-bar__wrapper', style: {flexGrow: 0}}}
                renderItem={autocomplete.renderItem || (item => item.label)}
                onSelect={val => autocomplete.onSelect(val)}
                onChange={e => handleChange(e.target.value)}
                value={value}
                items={effectiveItems}
            />
        );
    }

    // Default simple search bar without autocomplete
    return (
        <div className='search-bar__wrapper'>
            <div className={inputClassName}>
                <i className='fa fa-search' style={{marginRight: '9px', cursor: 'pointer'}} onClick={focusInput} />
                <input
                    ref={inputRef}
                    type='text'
                    className='argo-field'
                    placeholder={placeholder}
                    value={localValue}
                    onChange={e => handleChange(e.target.value)}
                    onFocus={e => {
                        setFocus(true);
                        e.target.select();
                    }}
                    onBlur={() => setFocus(false)}
                    onKeyDown={e => {
                        if (!disableKeyboardShortcuts && e.key === 'Escape' && inputRef.current) {
                            e.preventDefault();
                            inputRef.current.blur();
                        }
                    }}
                    style={{fontSize: '14px', flex: 1, minWidth: 0}}
                />
                <div className='keyboard-hint'>/</div>
                {localValue && <i className='fa fa-times' onClick={() => handleChange('')} style={{cursor: 'pointer', marginLeft: '5px'}} />}
            </div>
        </div>
    );
};
