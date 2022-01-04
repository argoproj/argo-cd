import * as models from '../models';
import * as React from 'react';
import {Key, KeybindingContext} from 'argo-ui/v2';
import {ContextApis} from '../context';
import {Autocomplete} from 'argo-ui';

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

export const SearchBar = (props: {content: string; ctx: ContextApis; apps: models.Application[]; currentApp?: string}) => {
    const {content, ctx, apps, currentApp} = {...props};
    const currentAppLabel = `${currentApp} (current app)`;
    const appList = apps.map(app => (currentApp === app.metadata.name ? currentAppLabel : app.metadata.name));

    const searchBar = React.useRef<HTMLDivElement>(null);

    const query = new URLSearchParams(window.location.search);
    const appInput = tryJsonParse(query.get('new'));

    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);

    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (searchBar.current && !appInput) {
                searchBar.current.querySelector('input').focus();
                setFocus(true);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (searchBar.current && !appInput && isFocused) {
                searchBar.current.querySelector('input').blur();
                setFocus(false);
                return true;
            }
            return false;
        }
    });

    return (
        <Autocomplete
            filterSuggestions={true}
            renderInput={inputProps => (
                <div className='applications-list__search' ref={searchBar}>
                    <i
                        className='fa fa-search'
                        style={{marginRight: '9px', cursor: 'pointer'}}
                        onClick={() => {
                            if (searchBar.current) {
                                searchBar.current.querySelector('input').focus();
                            }
                        }}
                    />
                    <input
                        {...inputProps}
                        onFocus={e => {
                            e.target.select();
                            if (inputProps.onFocus) {
                                inputProps.onFocus(e);
                            }
                        }}
                        style={{fontSize: '14px'}}
                        className='argo-field'
                        placeholder='Search applications...'
                    />
                    <div className='keyboard-hint'>/</div>
                    {content && (
                        <i className='fa fa-times' onClick={() => ctx.navigation.goto('.', {search: null}, {replace: true})} style={{cursor: 'pointer', marginLeft: '5px'}} />
                    )}
                </div>
            )}
            wrapperProps={{className: 'applications-list__search-wrapper'}}
            renderItem={item => (
                <React.Fragment>
                    <i className='icon argo-icon-application' /> {item.label}
                </React.Fragment>
            )}
            onSelect={val => {
                if (val !== currentAppLabel) {
                    ctx.navigation.goto(`/applications/${val}`);
                }
            }}
            onChange={e => ctx.navigation.goto('.', {search: e.target.value}, {replace: true})}
            value={content || ''}
            items={appList}
        />
    );
};
