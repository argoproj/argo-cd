import {DropDown} from 'argo-ui';
import {Key} from 'argo-ui/v2';
import classNames from 'classnames';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {getAppUrl} from '../utils';

// take top results after filtering to avoid performance issues
const MAX_RESULTS = 100;

function resourceIconClass(objectListKind: string): string {
    return objectListKind === 'applicationset' ? 'argo-icon-applicationset' : 'argo-icon-application';
}

export const ApplicationsDetailsAppDropdown = (props: {appName: string; objectListKind: string}) => {
    const [opened, setOpened] = React.useState(false);
    const [appFilter, setAppFilter] = React.useState('');
    const [apps, setApps] = React.useState<models.AbstractApplication[]>(null);
    const [loadFailed, setLoadFailed] = React.useState(false);
    const [highlighted, setHighlighted] = React.useState(-1);
    const {navigation} = React.useContext(Context);
    const dropdown = React.useRef<DropDown>(null);
    const anchorRef = React.useRef<HTMLSpanElement>(null);
    const inputRef = React.useRef<HTMLInputElement>(null);
    const activeItemRef = React.useRef<HTMLLIElement>(null);
    const listId = React.useId();
    const {appName, objectListKind} = props;

    const filteredApps = React.useMemo(
        () => (apps || []).filter(app => appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase())).slice(0, MAX_RESULTS),
        [apps, appFilter]
    );
    // derived rather than stored, so that a filter change can never leave the highlight pointing past the end of the list
    const activeIndex = Math.min(highlighted, filteredApps.length - 1);

    // refetched on every open, as the DataLoader this replaced was; the previous list stays on screen meanwhile
    React.useEffect(() => {
        if (!opened) {
            return;
        }
        let cancelled = false;
        services.applications
            .list([], objectListKind, {fields: ['items.metadata.name', 'items.metadata.namespace']})
            .then(list => {
                if (!cancelled) {
                    setApps(list.items);
                }
            })
            .catch(() => {
                if (!cancelled) {
                    setLoadFailed(true);
                }
            });
        return () => {
            cancelled = true;
        };
    }, [opened, objectListKind]);

    React.useEffect(() => {
        if (opened) {
            inputRef.current?.focus();
        }
    }, [opened]);

    // the menu itself is the scroll container (argo-ui gives .argo-dropdown__content.is-menu overflow: auto)
    React.useEffect(() => {
        if (activeIndex >= 0 && activeItemRef.current) {
            activeItemRef.current.scrollIntoView({block: 'nearest'});
        }
    }, [activeIndex]);

    const selectApp = (app: models.AbstractApplication) => {
        dropdown.current?.close();
        navigation.goto(`/${getAppUrl(app)}`);
    };

    const onFilterKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'ArrowDown' || e.keyCode === Key.DOWN) {
            e.preventDefault();
            e.stopPropagation();
            setHighlighted(Math.min(activeIndex + 1, filteredApps.length - 1));
        } else if (e.key === 'ArrowUp' || e.keyCode === Key.UP) {
            e.preventDefault();
            e.stopPropagation();
            setHighlighted(activeIndex < 0 ? filteredApps.length - 1 : Math.max(activeIndex - 1, 0));
        } else if (e.key === 'Enter' || e.keyCode === Key.ENTER) {
            // only ever acts on the row the user can see is highlighted
            if (activeIndex >= 0) {
                e.preventDefault();
                // argo-ui's DropDown clicks the first <li> of the menu on Enter; keep that handler out of this
                e.stopPropagation();
                selectApp(filteredApps[activeIndex]);
            }
        } else if (e.key === 'Escape' || e.keyCode === Key.ESCAPE) {
            e.preventDefault();
            e.stopPropagation();
            dropdown.current?.close();
            anchorRef.current?.focus();
        }
    };

    // DropDown renders the anchor as a component type, so an inline arrow would remount it — and drop focus — on every keystroke
    const anchor = React.useCallback(
        () => (
            <span
                ref={anchorRef}
                className='application-details-app-dropdown__anchor'
                role='button'
                tabIndex={0}
                aria-haspopup='listbox'
                onKeyDown={e => {
                    if (e.key === 'Enter' || e.keyCode === Key.ENTER || e.key === ' ') {
                        e.preventDefault();
                        e.stopPropagation();
                        e.currentTarget.click();
                    }
                }}>
                <i className='fa fa-search' /> <span>{appName}</span>
            </span>
        ),
        [appName]
    );

    return (
        <DropDown
            ref={dropdown}
            onOpenStateChange={open => {
                setOpened(open);
                // the anchor fires this again on every click while already open, which must not wipe what was typed
                if (open && !opened) {
                    setAppFilter('');
                    setHighlighted(-1);
                    setLoadFailed(false);
                }
            }}
            isMenu={true}
            anchor={anchor}>
            {opened && (
                <ul role='listbox' id={listId} aria-label={objectListKind === 'applicationset' ? 'Application sets' : 'Applications'}>
                    <li className='application-details-app-dropdown__filter' role='presentation'>
                        <span className='application-details-app-dropdown__filter-spacer' aria-hidden='true' />
                        <input
                            ref={inputRef}
                            className='argo-field'
                            value={appFilter}
                            role='combobox'
                            aria-expanded={true}
                            aria-controls={listId}
                            aria-autocomplete='list'
                            aria-activedescendant={activeIndex >= 0 ? `${listId}-option-${activeIndex}` : undefined}
                            aria-label={objectListKind === 'applicationset' ? 'Filter application sets' : 'Filter applications'}
                            onChange={e => {
                                setAppFilter(e.target.value);
                                // highlight the top match so that Enter always commits to a row the user can see
                                setHighlighted(e.target.value.length > 0 ? 0 : -1);
                            }}
                            onKeyDown={onFilterKeyDown}
                        />
                    </li>
                    {!apps && loadFailed && (
                        <li className='application-details-app-dropdown__status' role='presentation'>
                            Unable to load the list
                        </li>
                    )}
                    {!apps && !loadFailed && (
                        <li className='application-details-app-dropdown__status' role='presentation'>
                            Loading...
                        </li>
                    )}
                    {apps && filteredApps.length === 0 && (
                        <li className='application-details-app-dropdown__status' role='presentation'>
                            No matches
                        </li>
                    )}
                    {filteredApps.map((app, i) => (
                        <li
                            className={classNames('application-details-app-dropdown__item', {'application-details-app-dropdown__item--active': i === activeIndex})}
                            key={`${app.metadata.namespace}/${app.metadata.name}`}
                            id={`${listId}-option-${i}`}
                            role='option'
                            aria-selected={i === activeIndex}
                            ref={i === activeIndex ? activeItemRef : undefined}
                            onMouseMove={() => setHighlighted(i)}
                            onClick={() => selectApp(app)}>
                            <i className={`icon ${resourceIconClass(objectListKind)} resource-icon__font-icon application-details-app-dropdown__resource-icon`} />
                            <span>
                                {app.metadata.name}
                                {app.metadata.name === appName && ' (current)'}
                            </span>
                        </li>
                    ))}
                </ul>
            )}
        </DropDown>
    );
};
