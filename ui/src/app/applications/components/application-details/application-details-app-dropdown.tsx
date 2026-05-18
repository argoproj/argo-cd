import {DataLoader} from 'argo-ui';
import {Key, KeybindingContext} from 'argo-ui/v2';
import classNames from 'classnames';
import * as React from 'react';
import * as ReactDOM from 'react-dom';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {getTheme} from '../../../shared/utils';
import {getAppUrl} from '../utils';

export const ApplicationsDetailsAppDropdown = (props: {appName: string; objectListKind: string}) => {
    const [opened, setOpened] = React.useState(false);
    const [filter, setFilter] = React.useState('');
    const [highlight, setHighlight] = React.useState(0);
    const ctx = React.useContext(Context);
    const {useKeybinding} = React.useContext(KeybindingContext);
    const containerRef = React.useRef<HTMLDivElement>(null);
    const panelRef = React.useRef<HTMLDivElement>(null);
    const inputRef = React.useRef<HTMLInputElement>(null);
    const listRef = React.useRef<HTMLUListElement>(null);
    // Viewport coordinates for the portal-rendered panel; recomputed from the anchor's
    // bounding rect whenever the dropdown opens or the page scrolls/resizes.
    const [panelPos, setPanelPos] = React.useState<{top: number; left: number}>({top: 0, left: 0});
    // Latest filtered result, refreshed during render of the DataLoader child. Read by the
    // input's onKeyDown closure for Enter / Arrow navigation without re-computing the filter.
    const filteredRef = React.useRef<models.AbstractApplication[]>([]);

    const openDropdown = () => {
        setFilter('');
        setHighlight(0);
        setOpened(true);
    };

    React.useEffect(() => {
        if (opened && inputRef.current) {
            inputRef.current.focus();
        }
    }, [opened]);

    // Keep the keyboard-highlighted row visible when navigating past the scroll boundary.
    React.useEffect(() => {
        const active = listRef.current?.querySelector<HTMLLIElement>('.application-details-app-dropdown__item--active');
        active?.scrollIntoView({block: 'nearest'});
    }, [highlight, filter]);

    // Recompute panel position from the anchor's bounding rect. The panel is portal-rendered
    // into document.body to escape any ancestor `overflow: hidden`, so it needs absolute
    // viewport coordinates.
    const updatePanelPos = React.useCallback(() => {
        if (!containerRef.current) {
            return;
        }
        const rect = containerRef.current.getBoundingClientRect();
        setPanelPos({top: rect.bottom + 4, left: rect.left});
    }, []);

    // The portal panel sits at document.body, outside Layout's `.theme-*` wrapper, so
    // themify()'s descendant selectors don't match — re-apply the class on the portal root.
    const [theme, setTheme] = React.useState<string>('');
    React.useEffect(() => {
        const sub = services.viewPreferences.getPreferences().subscribe(p => setTheme(p.theme));
        return () => sub.unsubscribe();
    }, []);
    const themeClass = theme ? `theme-${getTheme(theme)}` : '';

    React.useLayoutEffect(() => {
        if (!opened) {
            return;
        }
        updatePanelPos();
        // Coalesce scroll/resize bursts into one update per frame to avoid
        // re-rendering the whole details view on every pixel of scroll.
        let raf = 0;
        const onScroll = () => {
            if (raf) {
                return;
            }
            raf = requestAnimationFrame(() => {
                raf = 0;
                updatePanelPos();
            });
        };
        window.addEventListener('scroll', onScroll, true);
        window.addEventListener('resize', onScroll);
        return () => {
            if (raf) {
                cancelAnimationFrame(raf);
            }
            window.removeEventListener('scroll', onScroll, true);
            window.removeEventListener('resize', onScroll);
        };
    }, [opened, updatePanelPos]);

    React.useEffect(() => {
        if (!opened) {
            return;
        }
        const handler = (e: MouseEvent) => {
            const target = e.target as Node;
            const insideAnchor = containerRef.current?.contains(target);
            const insidePanel = panelRef.current?.contains(target);
            if (!insideAnchor && !insidePanel) {
                setOpened(false);
            }
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, [opened]);

    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (!opened) {
                openDropdown();
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (opened) {
                setOpened(false);
                return true;
            }
            return false;
        }
    });

    const renderItems = (apps: models.AbstractApplication[]) => {
        const filtered = apps.filter(app => filter.length === 0 || app.metadata.name.toLowerCase().includes(filter.toLowerCase())).slice(0, 100); // take top 100 results after filtering to avoid performance issues
        filteredRef.current = filtered;
        const activeIndex = Math.min(highlight, Math.max(0, filtered.length - 1));
        if (filtered.length === 0) {
            return <li className='application-details-app-dropdown__empty'>No matches</li>;
        }
        return filtered.map((app, idx) => (
            <li
                key={`${app.metadata.namespace}/${app.metadata.name}`}
                className={classNames('application-details-app-dropdown__item', {
                    'application-details-app-dropdown__item--active': idx === activeIndex
                })}
                onMouseEnter={() => setHighlight(idx)}
                onClick={() => {
                    ctx.navigation.goto(`/${getAppUrl(app)}`);
                    setOpened(false);
                }}>
                {app.metadata.name}
                {app.metadata.name === props.appName && ' (current)'}
            </li>
        ));
    };

    return (
        <div className='application-details-app-dropdown' ref={containerRef}>
            <div className='application-details-app-dropdown__anchor' onClick={() => (opened ? setOpened(false) : openDropdown())}>
                <i className='fa fa-search' /> <span>{props.appName}</span>
            </div>
            {opened &&
                ReactDOM.createPortal(
                    <div className={themeClass}>
                        <div className='application-details-app-dropdown__panel' ref={panelRef} style={{top: panelPos.top, left: panelPos.left}}>
                            <div className='application-details-app-dropdown__search'>
                                <input
                                    ref={inputRef}
                                    className='argo-field'
                                    value={filter}
                                    placeholder='Filter applications...'
                                    onChange={e => {
                                        setFilter(e.target.value);
                                        setHighlight(0);
                                    }}
                                    onKeyDown={e => {
                                        const filtered = filteredRef.current;
                                        if (e.key === 'ArrowDown') {
                                            e.preventDefault();
                                            setHighlight(h => Math.min(h + 1, Math.max(0, filtered.length - 1)));
                                        } else if (e.key === 'ArrowUp') {
                                            e.preventDefault();
                                            setHighlight(h => Math.max(h - 1, 0));
                                        } else if (e.key === 'Enter' && filtered.length > 0) {
                                            e.preventDefault();
                                            const activeIndex = Math.min(highlight, filtered.length - 1);
                                            ctx.navigation.goto(`/${getAppUrl(filtered[activeIndex])}`);
                                            setOpened(false);
                                        } else if (e.key === 'Escape') {
                                            e.preventDefault();
                                            setOpened(false);
                                        }
                                    }}
                                />
                            </div>
                            <ul className='application-details-app-dropdown__list' ref={listRef}>
                                <DataLoader
                                    load={() => services.applications.list([], props.objectListKind, {fields: ['items.metadata.name', 'items.metadata.namespace']})}
                                    loadingRenderer={() => <li className='application-details-app-dropdown__empty'>Loading...</li>}>
                                    {apps => renderItems(apps.items)}
                                </DataLoader>
                            </ul>
                        </div>
                    </div>,
                    document.body
                )}
        </div>
    );
};
