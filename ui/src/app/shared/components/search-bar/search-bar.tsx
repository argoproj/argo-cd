import * as React from 'react';
import {Key, KeybindingContext} from 'argo-ui/v2';

import './search-bar.scss';

interface SearchBarProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    /** Disable keyboard shortcuts (useful when parent has custom handling) */
    disableKeyboardShortcuts?: boolean;
}

export const SearchBar: React.FC<SearchBarProps> = ({value, onChange, placeholder = 'Search...', disableKeyboardShortcuts = false}) => {
    const inputRef = React.useRef<HTMLInputElement>(null);
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

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (disableKeyboardShortcuts) {
            return;
        }
        // Handle Escape key to blur input
        if (e.key === 'Escape' && inputRef.current) {
            e.preventDefault();
            inputRef.current.blur();
        }
    };

    // Register global slash keybinding to focus search (when not already focused)
    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (disableKeyboardShortcuts || isFocused) {
                return false;
            }
            if (inputRef.current) {
                inputRef.current.focus();
                return true;
            }
            return false;
        }
    });

    return (
        <div className='search-bar__wrapper'>
            <div className='search-bar__input'>
                <i
                    className='fa fa-search'
                    style={{marginRight: '9px', cursor: 'pointer'}}
                    onClick={() => {
                        if (inputRef.current) {
                            inputRef.current.focus();
                        }
                    }}
                />
                <input
                    ref={inputRef}
                    type='text'
                    className='argo-field'
                    placeholder={placeholder}
                    value={localValue}
                    onChange={e => handleChange(e.target.value)}
                    onFocus={() => setFocus(true)}
                    onBlur={() => setFocus(false)}
                    onKeyDown={handleKeyDown}
                    style={{fontSize: '14px', flex: 1, minWidth: 0}}
                />
                <div className='keyboard-hint'>/</div>
                {localValue && <i className='fa fa-times' onClick={() => handleChange('')} style={{cursor: 'pointer', marginLeft: '5px'}} />}
            </div>
        </div>
    );
};
