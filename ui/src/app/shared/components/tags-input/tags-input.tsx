import * as classNames from 'classnames';

import {Autocomplete, AutocompleteApi, AutocompleteOption} from 'argo-ui';
import React, {FC, useRef, useState, MouseEvent, ChangeEvent} from 'react';

export interface TagsInputProps {
    tags: string[];
    autocomplete?: (AutocompleteOption | string)[];
    onChange?: (tags: string[]) => any;
    placeholder?: string;
}

require('./tags-input.scss');

export const TagsInput: FC<TagsInputProps> = ({tags: initialTags, autocomplete, onChange, placeholder}) => {
    const [tags, setTags] = useState(initialTags || []);
    const [input, setInput] = useState('');
    const [focused, setFocused] = useState(false);

    const inputEl = useRef<HTMLInputElement | null>(null);
    const autocompleteApi = useRef<AutocompleteApi | null>(null);

    const handleFocusInput = () => {
        inputEl.current?.focus();
    };

    const handleTagsChange = (newTags: string[]) => {
        onChange?.(newTags);
        if (autocompleteApi.current) {
            setTimeout(() => autocompleteApi.current.refresh());
        }
    };

    const removeTagByIndex = (index: number) => {
        if (index < 0 || index >= tags.length || tags.length === 0) {
            return;
        }
        const newTags = [...tags.slice(0, index), ...tags.slice(index + 1)];
        setTags(newTags);
        handleTagsChange(newTags);
    };

    const handleRemoveTag = (e: MouseEvent<HTMLElement>, index: number) => {
        removeTagByIndex(index);
        e.stopPropagation();
    };

    const handleChangeInput = (event: ChangeEvent<HTMLInputElement>) => {
        setInput(event.target.value);
    };

    const handleSelectTag = (value: string) => {
        if (tags.indexOf(value) === -1) {
            const newTags = tags.concat(value);
            setInput('');
            setTags(newTags);
            handleTagsChange(newTags);
        }
    };

    return (
        <div className={classNames('tags-input argo-field', {'tags-input--focused': focused || !!input})} onClick={handleFocusInput}>
            {tags.map((tag, index) => (
                <span className='tags-input__tag' key={tag}>
                    {tag} <i className='fa fa-times' onClick={e => handleRemoveTag(e, index)} />
                </span>
            ))}
            <Autocomplete
                filterSuggestions={true}
                autoCompleteRef={api => (autocompleteApi.current = api)}
                value={input}
                items={autocomplete}
                onChange={handleChangeInput}
                onSelect={handleSelectTag}
                renderInput={props => (
                    <input
                        {...props}
                        placeholder={placeholder}
                        ref={el => {
                            inputEl.current = el;
                            if (props.ref) {
                                (props.ref as any)(el);
                            }
                        }}
                        onFocus={e => {
                            if (props.onFocus) {
                                props.onFocus(e);
                            }
                            setFocused(true);
                        }}
                        onBlur={e => {
                            if (props.onBlur) {
                                props.onBlur(e);
                            }
                            setFocused(false);
                        }}
                        onKeyDown={e => {
                            if (e.key === 'Backspace' && input === '') {
                                removeTagByIndex(tags.length - 1);
                            }
                            if (props.onKeyDown) {
                                props.onKeyDown(e);
                            }
                        }}
                        onKeyUp={e => {
                            if (e.key === 'Enter' && input) {
                                handleSelectTag(input);
                            }
                            if (props.onKeyUp) {
                                props.onKeyUp(e);
                            }
                        }}
                    />
                )}
            />
        </div>
    );
};
