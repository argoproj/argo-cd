import * as React from 'react';
import {Autocomplete, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import {Key, KeybindingContext} from 'argo-ui/v2';

import './search-bar.scss';

interface SearchBarProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    /** Disable keyboard shortcuts (useful when parent has custom handling) */
    disableKeyboardShortcuts?: boolean;
    /** Optional regex toggle. When provided, a `.*` button is rendered next to the input
     *  and the input's border switches to a teal/red active/invalid state based on the value. */
    regex?: {
        enabled: boolean;
        onToggle: () => void;
    };
    /** Optional autocomplete configuration */
    autocomplete?: {
        items: Array<string | {value: string; label: string}>;
        onSelect: (value: string) => void;
        renderItem?: (item: {value: string; label: string}) => React.ReactNode;
        filterSuggestions?: boolean;
    };
}

export const SearchBar: React.FC<SearchBarProps> = ({value, onChange, placeholder = 'Search...', disableKeyboardShortcuts = false, regex, autocomplete}) => {
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

    const regexInvalid = React.useMemo(() => {
        if (!regex?.enabled || !value) {
            return false;
        }
        try {
            new RegExp(value);
            return false;
        } catch {
            return true;
        }
    }, [regex?.enabled, value]);

    const inputClassName = classNames('search-bar__input', {
        'search-bar__input--regex': regex?.enabled && !regexInvalid,
        'search-bar__input--regex-invalid': regexInvalid
    });

    const regexToggle = regex && (
        <Tooltip content={regex.enabled ? (regexInvalid ? 'Invalid regex pattern' : 'Regex search enabled, click to switch to plain text') : 'Click to enable regex search'}>
            <button
                type='button'
                aria-label='Toggle regex search'
                aria-pressed={regex.enabled}
                className={classNames('search-bar__regex-toggle', {
                    'search-bar__regex-toggle--active': regex.enabled && !regexInvalid,
                    'search-bar__regex-toggle--invalid': regexInvalid
                })}
                onClick={regex.onToggle}>
                .*
            </button>
        </Tooltip>
    );

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

        return (
            <Autocomplete
                filterSuggestions={autocomplete.filterSuggestions ?? true}
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
                        {regexToggle}
                        <div className='keyboard-hint'>/</div>
                        {value && <i className='fa fa-times' onClick={() => handleChange('')} style={{cursor: 'pointer', marginLeft: '5px'}} />}
                    </div>
                )}
                wrapperProps={{className: 'search-bar__wrapper', style: {flexGrow: 0}}}
                renderItem={autocomplete.renderItem || (item => item.label)}
                onSelect={val => autocomplete.onSelect(val)}
                onChange={e => handleChange(e.target.value)}
                value={value}
                items={normalizedItems}
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
                {regexToggle}
                <div className='keyboard-hint'>/</div>
                {localValue && <i className='fa fa-times' onClick={() => handleChange('')} style={{cursor: 'pointer', marginLeft: '5px'}} />}
            </div>
        </div>
    );
};
