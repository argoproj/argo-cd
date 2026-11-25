import * as React from 'react';
import {useCombobox} from 'downshift';
import {ApplicationQueryValueIndex, completeApplicationQuery} from './applications-query-completion';

export const ApplicationsQueryInput = ({
    value,
    onChange,
    values,
    invalid
}: {
    value: string;
    onChange: (value: string) => void;
    values?: ApplicationQueryValueIndex;
    invalid: boolean;
}) => {
    const inputRef = React.useRef<HTMLInputElement>(null);
    const [cursor, setCursor] = React.useState(value.length);
    const pendingCursor = React.useRef<number | null>(null);
    const attachInput = React.useCallback((node: HTMLInputElement | null) => {
        inputRef.current = node;
    }, []);
    const suggestions = React.useMemo(() => completeApplicationQuery(value, cursor, values), [value, cursor, values]);
    const accept = (item: (typeof suggestions)[number]) => {
        onChange(item.value);
        setCursor(item.cursor);
        pendingCursor.current = item.cursor;
    };
    React.useLayoutEffect(() => {
        if (pendingCursor.current !== null) {
            inputRef.current?.focus();
            inputRef.current?.setSelectionRange(pendingCursor.current, pendingCursor.current);
            pendingCursor.current = null;
        }
    });
    const {isOpen, highlightedIndex, getInputProps, getMenuProps, getItemProps, openMenu, closeMenu} = useCombobox({
        id: 'application-query-completion',
        inputId: 'application-query',
        labelId: 'application-query-label',
        items: suggestions,
        inputValue: value,
        selectedItem: null,
        itemToString: item => item?.value || '',
        stateReducer: (_state, {type, changes}) => (type === useCombobox.stateChangeTypes.InputClick ? {...changes, isOpen: true} : changes),
        onSelectedItemChange: ({selectedItem}) => {
            if (selectedItem) accept(selectedItem);
        }
    });
    const visible = isOpen && suggestions.length > 0;

    return (
        <div className={`search-bar__input applications-query__input${invalid ? ' applications-query__input--invalid' : ''}`}>
            <i className='fa fa-search' aria-hidden='true' />
            <input
                // Downshift forwards this callback ref to the input; it does not read it during render.
                // eslint-disable-next-line react-hooks/refs
                {...getInputProps({
                    'ref': attachInput,
                    'className': 'argo-field',
                    'onChange': event => {
                        setCursor(event.target.selectionStart ?? event.target.value.length);
                        onChange(event.target.value);
                    },
                    'onFocus': event => {
                        setCursor(event.target.selectionStart ?? value.length);
                        openMenu();
                    },
                    'onSelect': event => setCursor(event.currentTarget.selectionStart ?? value.length),
                    'onKeyDown': event => {
                        if (event.nativeEvent.isComposing) {
                            if (event.key === 'Enter') event.preventDefault();
                            (event as any).preventDownshiftDefault = true;
                            return;
                        }
                        if (event.key === 'Tab' && !event.shiftKey && visible) {
                            event.preventDefault();
                            accept(suggestions[highlightedIndex < 0 ? 0 : highlightedIndex]);
                            closeMenu();
                        }
                        if (event.key === 'Enter' && (!isOpen || highlightedIndex < 0)) {
                            // Let the form apply the query when no suggestion was chosen.
                            (event as any).preventDownshiftDefault = true;
                            closeMenu();
                        }
                    },
                    'placeholder': 'sync = OutOfSync OR health = Degraded',
                    'aria-invalid': invalid,
                    'aria-describedby': invalid ? 'application-query-error' : 'application-query-status',
                    'aria-expanded': visible,
                    'autoComplete': 'off',
                    'spellCheck': false
                })}
            />
            <div {...getMenuProps({'aria-label': 'Query suggestions', 'className': 'applications-query__suggestions', 'hidden': !visible})}>
                {visible &&
                    suggestions.map((item, index) => (
                        <div
                            {...getItemProps({item, index})}
                            key={item.label}
                            className={`select__option${highlightedIndex === index ? ' selected' : ''}`}
                            onMouseDown={event => event.preventDefault()}>
                            <span>{item.label}</span>
                            <span className='applications-query__suggestion-description'>{item.description}</span>
                        </div>
                    ))}
            </div>
        </div>
    );
};
