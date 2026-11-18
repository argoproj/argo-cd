import classNames from 'classnames';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import {Autocomplete, AutocompleteApi, AutocompleteOption} from 'argo-ui';

export interface TagsInputProps {
    tags: string[];
    autocomplete?: (AutocompleteOption | string)[];
    onChange?: (tags: string[]) => any;
    placeholder?: string;
    description?: string;
    ariaLabel?: string;
}

interface TagsInputState {
    tags: string[];
    input: string;
    focused: boolean;
}

require('./tags-input.scss');

export function TagsInput(props: TagsInputProps) {
    const [state, setState] = useState<TagsInputState>({tags: props.tags || [], input: '', focused: false});

    const descriptionId = React.useId();
    const inputEl = useRef<HTMLInputElement | null>(null);
    const autocompleteApi = useRef<AutocompleteApi | null>(null);
    const pendingBlurInput = useRef<string | null>(null);

    const onTagsChange = (tags: string[]) => {
        if (props.onChange) {
            props.onChange(tags);
            if (autocompleteApi.current) {
                setTimeout(() => autocompleteApi.current.refresh());
            }
        }
    };

    const addTag = (tag: string, focused = state.focused) => {
        if (state.tags.includes(tag)) {
            setState({...state, input: '', focused});
            return;
        }
        const newTags = state.tags.concat(tag);
        setState({...state, input: '', focused, tags: newTags});
        onTagsChange(newTags);
    };

    useEffect(() => {
        // Autocomplete selects a highlighted suggestion in its effect after blur.
        // Commit free text only if that selection did not consume the input.
        const input = pendingBlurInput.current;
        pendingBlurInput.current = null;
        if (input) {
            addTag(input, false);
        }
    });

    return (
        <div className={classNames('tags-input argo-field', {'tags-input--focused': state.focused || !!state.input})} onClick={() => inputEl.current && inputEl.current.focus()}>
            {state.tags.map((tag, i) => (
                <span className='tags-input__tag' key={tag}>
                    {tag}{' '}
                    <i
                        className='fa fa-times'
                        onClick={e => {
                            const newTags = [...state.tags.slice(0, i), ...state.tags.slice(i + 1)];
                            setState(prevState => ({...prevState, tags: newTags}));
                            onTagsChange(newTags);
                            e.stopPropagation();
                        }}
                    />
                </span>
            ))}
            <Autocomplete
                filterSuggestions={true}
                autoCompleteRef={api => (autocompleteApi.current = api)}
                value={state.input}
                items={props.autocomplete}
                onChange={e => setState({...state, input: e.target.value})}
                onSelect={value => {
                    pendingBlurInput.current = null;
                    addTag(value);
                }}
                renderInput={inputProps => (
                    <input
                        {...inputProps}
                        placeholder={props.placeholder}
                        aria-label={props.ariaLabel}
                        aria-describedby={props.description ? descriptionId : undefined}
                        ref={el => {
                            inputEl.current = el;
                            if (inputProps.ref) {
                                (inputProps.ref as any)(el);
                            }
                        }}
                        onFocus={e => {
                            pendingBlurInput.current = null;
                            if (inputProps.onFocus) {
                                inputProps.onFocus(e);
                            }
                            setState({...state, focused: true});
                        }}
                        onBlur={e => {
                            pendingBlurInput.current = state.input;
                            if (inputProps.onBlur) {
                                inputProps.onBlur(e);
                            }
                            setState({...state, focused: false});
                        }}
                        onKeyDown={e => {
                            if (e.keyCode === 8 && state.tags.length > 0 && state.input === '') {
                                const newTags = state.tags.slice(0, state.tags.length - 1);
                                setState(prevState => ({...prevState, tags: newTags}));
                                onTagsChange(newTags);
                            }
                            if (inputProps.onKeyDown) {
                                inputProps.onKeyDown(e);
                            }
                        }}
                        onKeyUp={e => {
                            if (e.keyCode === 13 && state.input) {
                                addTag(state.input);
                            }
                            if (inputProps.onKeyUp) {
                                inputProps.onKeyUp(e);
                            }
                        }}
                    />
                )}
            />
            {props.description && (
                <span id={descriptionId} hidden>
                    {props.description}
                </span>
            )}
        </div>
    );
}
