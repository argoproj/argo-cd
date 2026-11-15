import classNames from 'classnames';
import * as React from 'react';

export interface KindFilterSelectOption {
    value: string;
    title: string;
}

export type KindOptionInput = KindFilterSelectOption | string;

export interface KindFilterSelectProps {
    id?: string;
    value: string;
    options: KindOptionInput[];
    onChange: (option: KindFilterSelectOption) => void;
    placeholder?: string;
}

const normalizeOptions = (options: KindOptionInput[]): KindFilterSelectOption[] => {
    return options.map(item => (typeof item === 'string' ? {value: item, title: item} : item));
};

export const KindFilterSelect: React.FC<KindFilterSelectProps> = ({id = 'application-sync-panel-kind-filter-select', value, options, onChange, placeholder = 'All kinds'}) => {
    const [isOpen, setIsOpen] = React.useState(false);
    const [search, setSearch] = React.useState('');
    const [highlightedIndex, setHighlightedIndex] = React.useState(0);
    const containerRef = React.useRef<HTMLDivElement>(null);
    const searchInputRef = React.useRef<HTMLInputElement>(null);
    const triggerRef = React.useRef<HTMLButtonElement>(null);

    const normalized = React.useMemo(() => normalizeOptions(options), [options]);

    const filteredOptions = React.useMemo(() => {
        if (!search) {
            return normalized;
        }
        const lowerSearch = search.toLowerCase();
        return normalized.filter(opt => opt.title.toLowerCase().includes(lowerSearch));
    }, [normalized, search]);

    const activeIndex = highlightedIndex >= 0 && highlightedIndex < filteredOptions.length ? highlightedIndex : 0;

    // The option list scrolls (`.select__options` caps its height), so keep the option Enter would select in view.
    React.useEffect(() => {
        if (!isOpen || filteredOptions.length === 0) {
            return;
        }
        const activeOption = filteredOptions[activeIndex];
        document.getElementById(`${id}-opt-${activeOption.value || 'all'}`)?.scrollIntoView?.({block: 'nearest'});
    }, [activeIndex, isOpen, filteredOptions, id]);

    React.useEffect(() => {
        if (!isOpen) {
            return;
        }
        const handleClickOutside = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setIsOpen(false);
                setSearch('');
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [isOpen]);

    const openDropdown = () => {
        setIsOpen(true);
        setTimeout(() => {
            if (searchInputRef.current) {
                searchInputRef.current.focus();
            }
        }, 0);
    };

    const selectOption = (option: KindFilterSelectOption) => {
        onChange(option);
        setIsOpen(false);
        setSearch('');
        if (triggerRef.current) {
            triggerRef.current.focus();
        }
    };

    const handleTriggerKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            openDropdown();
        }
    };

    const handleSearchKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            setHighlightedIndex(prev => (filteredOptions.length > 0 ? (prev + 1) % filteredOptions.length : 0));
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setHighlightedIndex(prev => (filteredOptions.length > 0 ? (prev - 1 + filteredOptions.length) % filteredOptions.length : 0));
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (filteredOptions.length > 0 && activeIndex >= 0 && activeIndex < filteredOptions.length) {
                selectOption(filteredOptions[activeIndex]);
            }
        } else if (e.key === 'Escape') {
            e.preventDefault();
            setIsOpen(false);
            setSearch('');
            if (triggerRef.current) {
                triggerRef.current.focus();
            }
        } else if (e.key === 'Tab') {
            setIsOpen(false);
            setSearch('');
        }
    };

    const selectedOption = normalized.find(opt => opt.value === value);
    const displayText = selectedOption?.title || placeholder;

    return (
        <div className='select kind-filter-select' ref={containerRef}>
            <button
                type='button'
                id={id}
                ref={triggerRef}
                className='select__value kind-filter-select__trigger'
                onClick={() => (isOpen ? setIsOpen(false) : openDropdown())}
                onKeyDown={handleTriggerKeyDown}
                aria-expanded={isOpen}
                aria-haspopup='listbox'
                aria-controls={`${id}-listbox`}
                aria-label='Filter by Kind'>
                <span className='kind-filter-select__title'>{displayText}</span>
                <span className='select__value-arrow'>
                    <i className='argo-icon-expand-arrow' />
                </span>
            </button>
            <div className={classNames('select__options', {opened: isOpen})}>
                <input
                    ref={searchInputRef}
                    type='text'
                    className='select__search'
                    placeholder='Search'
                    value={search}
                    onChange={e => {
                        if (!isOpen) {
                            setIsOpen(true);
                        }
                        setSearch(e.target.value);
                    }}
                    onFocus={() => {
                        if (!isOpen) {
                            setIsOpen(true);
                        }
                    }}
                    onKeyDown={handleSearchKeyDown}
                    role='combobox'
                    aria-label='Filter by Kind'
                    aria-expanded={isOpen}
                    aria-activedescendant={isOpen && filteredOptions[activeIndex] ? `${id}-opt-${filteredOptions[activeIndex].value || 'all'}` : undefined}
                    aria-autocomplete='list'
                    aria-controls={`${id}-listbox`}
                />
                <div role='listbox' id={`${id}-listbox`} aria-label='Available kinds'>
                    {filteredOptions.map((opt, idx) => {
                        const isSelected = opt.value === value;
                        const isHighlighted = idx === activeIndex;
                        const optId = `${id}-opt-${opt.value || 'all'}`;
                        return (
                            <div
                                key={opt.value}
                                id={optId}
                                role='option'
                                aria-selected={isSelected}
                                className={classNames('select__option', {
                                    'selected': isSelected,
                                    'kind-filter-select__option--highlighted': isHighlighted
                                })}
                                onMouseEnter={() => setHighlightedIndex(idx)}
                                onClick={() => selectOption(opt)}>
                                <span>{opt.title}</span>
                            </div>
                        );
                    })}
                    {filteredOptions.length === 0 && <div className='select__empty'>No results</div>}
                </div>
            </div>
        </div>
    );
};
