import * as classNames from 'classnames';
import React, {useRef, useState} from 'react';

import {Autocomplete, AutocompleteApi, AutocompleteOption} from 'argo-ui';

export interface TagsInputProps {
    tags: string[];
    autocomplete?: (AutocompleteOption | string)[];
    onChange?: (tags: string[]) => any;
    placeholder?: string;
}

interface TagsInputState {
    tags: string[];
    input: string;
    focused: boolean;
}

require('./tags-input.scss');

export function TagsInput(props: TagsInputProps) {
    const [state, setState] = useState<TagsInputState>({tags: props.tags || [], input: '', focused: false});

    const inputEl = useRef<HTMLInputElement | null>(null);
    const autocompleteApi = useRef<AutocompleteApi | null>(null);

    const onTagsChange = (tags: string[]) => {
        if (props.onChange) {
            props.onChange(tags);
            if (autocompleteApi.current) {
                setTimeout(() => autocompleteApi.current.refresh());
            }
        }
    };

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
                    if (state.tags.indexOf(value) === -1) {
                        const newTags = state.tags.concat(value);
                        setState(prevState => ({...prevState, input: '', tags: newTags}));
                        onTagsChange(newTags);
                    }
                }}
                renderInput={props => (
                    <input
                        {...props}
                        placeholder={props.placeholder}
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
                            setState({...state, focused: true});
                        }}
                        onBlur={e => {
                            if (props.onBlur) {
                                props.onBlur(e);
                            }
                            setState({...state, focused: false});
                        }}
                        onKeyDown={e => {
                            if (e.keyCode === 8 && state.tags.length > 0 && state.input === '') {
                                const newTags = state.tags.slice(0, state.tags.length - 1);
                                setState(prevState => ({...prevState, tags: newTags}));
                                onTagsChange(newTags);
                            }
                            if (props.onKeyDown) {
                                props.onKeyDown(e);
                            }
                        }}
                        onKeyUp={e => {
                            if (e.keyCode === 13 && state.input && state.tags.indexOf(state.input) === -1) {
                                const newTags = state.tags.concat(state.input);
                                setState(prevState => ({...prevState, input: '', tags: newTags}));
                                onTagsChange(newTags);
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
}
