import * as React from 'react';
import {compileApplicationQuery} from './applications-query';
import {applicationQueryLink} from './applications-query-url';
import {ApplicationQueryValueIndex} from './applications-query-completion';
import {ApplicationsQueryInput} from './applications-query-input';
import './applications-query-bar.scss';

export const ApplicationsQueryBar = ({value, onSearch, total, values}: {value: string; onSearch: (query: string) => void; total: number; values?: ApplicationQueryValueIndex}) => {
    const [draft, setDraft] = React.useState(value);
    const [previous, setPrevious] = React.useState(value);
    const [shareStatus, setShareStatus] = React.useState('');
    if (previous !== value) {
        setPrevious(value);
        setDraft(value);
        setShareStatus('');
    }
    const {error} = React.useMemo(() => compileApplicationQuery(draft), [draft]);
    const changed = draft !== value;
    const search = () => {
        if (!error) {
            setDraft(draft.trim());
            setShareStatus('');
            onSearch(draft.trim());
        }
    };
    const copyLink = async () => {
        if (changed) {
            setDraft(draft.trim());
            onSearch(draft.trim());
        }
        try {
            await navigator.clipboard.writeText(applicationQueryLink(window.location.href, draft.trim()));
            setShareStatus('Link copied');
        } catch {
            setShareStatus('Could not copy. Apply the query and copy the browser address.');
        }
    };

    return (
        <section className='applications-query' aria-label='Application query'>
            <form
                onSubmit={event => {
                    event.preventDefault();
                    search();
                }}>
                <label id='application-query-label' htmlFor='application-query'>
                    Search applications
                </label>
                <div className='applications-query__row'>
                    <ApplicationsQueryInput
                        value={draft}
                        values={values}
                        invalid={!!error}
                        onChange={text => {
                            setDraft(text);
                            setShareStatus('');
                        }}
                    />
                    <button type='submit' className='argo-button argo-button--base' disabled={!!error}>
                        Search
                    </button>
                    <button
                        type='button'
                        className='argo-button argo-button--base-o'
                        disabled={!draft && !value}
                        onClick={() => {
                            setDraft('');
                            setShareStatus('');
                            onSearch('');
                        }}>
                        Clear
                    </button>
                    <button type='button' className='argo-button argo-button--base-o' disabled={!!error} onClick={copyLink}>
                        <i className='fa fa-link' /> Copy link
                    </button>
                </div>
            </form>
            {error && (
                <div id='application-query-error' className='applications-query__error' role='alert'>
                    At character {error.position}: {error.message}
                </div>
            )}
            <div id='application-query-status' className='applications-query__status' role='status'>
                {shareStatus || (changed ? 'Press Enter or Search to apply.' : `${total} matching application${total === 1 ? '' : 's'}`)}
            </div>
        </section>
    );
};
