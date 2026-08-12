import {DataLoader, DropDownMenu} from 'argo-ui';

import * as React from 'react';
import {services, ViewPreferences} from '../../services';

require('./paginate.scss');

// We use a custom paginator instead of react-paginate so the control keeps a stable width
// as the current page changes. react-paginate adapts its layout (ellipsis collapse, margin/range
// overlap near the edges) and renders a variable number of page links, which shifts the Next
// button and makes repeated Next clicks harder. getPaginationSlots() always renders the same
// number of slots (first page, last page, ellipsis, and a sliding window) when pageCount > 7.
const BREAK_JUMP = 5;
const FIXED_SLOT_COUNT = 7;

export type PaginationSlot = {type: 'page'; page: number} | {type: 'break'; direction: 'backward' | 'forward'};

export function getPaginationSlots(currentPage: number, pageCount: number): PaginationSlot[] {
    if (pageCount <= 1) {
        return [];
    }
    if (pageCount <= FIXED_SLOT_COUNT) {
        return Array.from({length: pageCount}, (_, i) => ({type: 'page', page: i}));
    }

    const last = pageCount - 1;
    const nearStart = currentPage <= 3;
    const nearEnd = currentPage >= last - 3;

    if (nearStart) {
        return [
            {type: 'page', page: 0},
            {type: 'page', page: 1},
            {type: 'page', page: 2},
            {type: 'page', page: 3},
            {type: 'page', page: 4},
            {type: 'break', direction: 'forward'},
            {type: 'page', page: last}
        ];
    }
    if (nearEnd) {
        return [
            {type: 'page', page: 0},
            {type: 'break', direction: 'backward'},
            {type: 'page', page: last - 4},
            {type: 'page', page: last - 3},
            {type: 'page', page: last - 2},
            {type: 'page', page: last - 1},
            {type: 'page', page: last}
        ];
    }
    return [
        {type: 'page', page: 0},
        {type: 'break', direction: 'backward'},
        {type: 'page', page: currentPage - 1},
        {type: 'page', page: currentPage},
        {type: 'page', page: currentPage + 1},
        {type: 'break', direction: 'forward'},
        {type: 'page', page: last}
    ];
}

interface PaginatorProps {
    page: number;
    pageCount: number;
    pageNumMinWidth: string;
    onPageChange: (page: number) => void;
}

function Paginator({page, pageCount, pageNumMinWidth, onPageChange}: PaginatorProps) {
    const slots = getPaginationSlots(page, pageCount);
    const isFirst = page === 0;
    const isLast = page === pageCount - 1;

    function handleBreakClick(direction: 'backward' | 'forward') {
        if (direction === 'backward') {
            onPageChange(Math.max(0, page - BREAK_JUMP));
        } else {
            onPageChange(Math.min(pageCount - 1, page + BREAK_JUMP));
        }
    }

    return (
        <ul className='paginate__paginator' style={{'--paginate-page-min-width': pageNumMinWidth} as React.CSSProperties}>
            <li className={`previous${isFirst ? ' disabled' : ''}`}>
                <a role='button' tabIndex={isFirst ? -1 : 0} aria-disabled={isFirst} onClick={() => !isFirst && onPageChange(page - 1)}>
                    Previous
                </a>
            </li>
            {slots.map((slot, i) => {
                if (slot.type === 'break') {
                    return (
                        <li key={`break-${i}`} className='break'>
                            <a
                                role='button'
                                className='paginate__break-link'
                                tabIndex={0}
                                aria-label={slot.direction === 'backward' ? 'Jump backward' : 'Jump forward'}
                                onClick={() => handleBreakClick(slot.direction)}>
                                ...
                            </a>
                        </li>
                    );
                }
                return (
                    <li key={`page-${slot.page}`} className={slot.page === page ? 'selected' : undefined}>
                        <a
                            role='button'
                            className='paginate__page-link'
                            tabIndex={slot.page === page ? -1 : 0}
                            aria-current={slot.page === page ? 'page' : undefined}
                            onClick={() => slot.page !== page && onPageChange(slot.page)}>
                            {slot.page + 1}
                        </a>
                    </li>
                );
            })}
            <li className={`next${isLast ? ' disabled' : ''}`}>
                <a role='button' tabIndex={isLast ? -1 : 0} aria-disabled={isLast} onClick={() => !isLast && onPageChange(page + 1)}>
                    Next
                </a>
            </li>
        </ul>
    );
}

export interface SortOption<T> {
    title: string;
    compare: (a: T, b: T) => number;
    defaultDirection?: 'asc' | 'desc';
}

export interface PaginateProps<T> {
    page: number;
    onPageChange: (page: number) => any;
    children: (data: T[]) => React.ReactNode;
    data: T[];
    emptyState?: () => React.ReactNode;
    preferencesKey?: string;
    header?: React.ReactNode;
    showHeader?: boolean;
    sortOptions?: SortOption<T>[];
    /** Resource key (e.g. from `?highlight=`) of the row to show after navigation. */
    focusItemKey?: string;
    /** Maps each list item to the same key format as `focusItemKey`. */
    getItemKey?: (item: T) => string;
}

export function Paginate<T>({page, onPageChange, children, data, emptyState, preferencesKey, header, showHeader, sortOptions, focusItemKey, getItemKey}: PaginateProps<T>) {
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => (
                <PaginateContent
                    page={page}
                    onPageChange={onPageChange}
                    data={data}
                    emptyState={emptyState}
                    preferencesKey={preferencesKey || 'default'}
                    header={header}
                    showHeader={showHeader}
                    sortOptions={sortOptions}
                    focusItemKey={focusItemKey}
                    getItemKey={getItemKey}
                    pref={pref}>
                    {children}
                </PaginateContent>
            )}
        </DataLoader>
    );
}

function PaginateContent<T>({
    page,
    onPageChange,
    children,
    data,
    emptyState,
    preferencesKey,
    header,
    showHeader,
    sortOptions,
    focusItemKey,
    getItemKey,
    pref
}: PaginateProps<T> & {pref: ViewPreferences}) {
    const pageSize = pref.pageSizes[preferencesKey] || 10;
    const sortOption = sortOptions ? (pref.sortOptions && pref.sortOptions[preferencesKey]) || sortOptions[0].title : '';
    const pageCount = pageSize === -1 ? 1 : Math.ceil(data.length / pageSize);
    const currentPage = pageCount <= page ? pageCount - 1 : page;

    const sortedData = React.useMemo(() => {
        const next = [...data];
        if (sortOption && sortOptions) {
            const selectedSort = sortOptions.find(o => o.title === sortOption);
            if (selectedSort) {
                const direction = pref.sortDirections?.[preferencesKey] ?? selectedSort.defaultDirection ?? 'asc';
                next.sort((a, b) => {
                    const result = selectedSort.compare(a, b);
                    return direction === 'asc' ? result : -result;
                });
            }
        }
        return next;
    }, [data, sortOption, sortOptions, pref.sortDirections, preferencesKey]);

    // Deep-link highlight: when `focusItemKey` is set (e.g. opening an Application from the
    // Resources page with `?highlight=group/kind/namespace/name`), keep the displayed page in
    // sync with the page that contains that item. We intentionally re-evaluate whenever the data
    // changes rather than latching a one-shot flag: the resource tree streams in incrementally, so
    // the item's index (and therefore its page) can change after the first render. We stop forcing
    // the page only once the user manually paginates (`userPaginatedForKey`), and reset on key change.
    const userPaginatedForKey = React.useRef<string | null>(null);
    const getItemKeyRef = React.useRef(getItemKey);
    const onPageChangeRef = React.useRef(onPageChange);
    const currentPageRef = React.useRef(currentPage);

    // Keep the latest values in refs so the highlight effect below can read them without
    // listing them as dependencies (which would make it re-run on every render). Refs must be
    // mutated after render, so we sync them in an effect declared before that one.
    React.useEffect(() => {
        getItemKeyRef.current = getItemKey;
        onPageChangeRef.current = onPageChange;
        currentPageRef.current = currentPage;
    });

    React.useEffect(() => {
        if (!focusItemKey || !getItemKeyRef.current) {
            userPaginatedForKey.current = null;
            return;
        }
        if (userPaginatedForKey.current && userPaginatedForKey.current !== focusItemKey) {
            userPaginatedForKey.current = null;
        }
        if (userPaginatedForKey.current === focusItemKey) {
            return;
        }
        const index = sortedData.findIndex(item => getItemKeyRef.current!(item) === focusItemKey);
        if (index < 0) {
            return;
        }
        const targetPage = pageSize === -1 ? 0 : Math.floor(index / pageSize);
        if (targetPage < 0 || targetPage >= pageCount) {
            return;
        }
        // Only navigate when needed; comparing against the live page avoids a render loop.
        if (targetPage !== currentPageRef.current) {
            onPageChangeRef.current(targetPage);
        }
    }, [focusItemKey, pageCount, pageSize, sortedData, sortOption]);

    const handlePageChange = React.useCallback(
        (newPage: number) => {
            // User took over pagination; stop auto-jumping for this highlight key.
            if (focusItemKey) {
                userPaginatedForKey.current = focusItemKey;
            }
            onPageChange(newPage);
        },
        [focusItemKey, onPageChange]
    );

    function paginator() {
        const pageNumMinWidth = `${Math.max(2, String(pageCount).length)}ch`;
        return (
            <div style={{marginBottom: '0.5em'}}>
                <div style={{display: 'flex', alignItems: 'center', marginBottom: '0.5em'}}>
                    {pageCount > 1 && <Paginator page={currentPage} pageCount={pageCount} pageNumMinWidth={pageNumMinWidth} onPageChange={handlePageChange} />}
                    <div className='paginate__size-menu'>
                        {sortOptions && (
                            <DropDownMenu
                                qeId={`paginate-sort-${preferencesKey}`}
                                anchor={() => (
                                    <a onMouseDown={() => document.body.click()}>
                                        Sort: {sortOption.toLowerCase()} <i className='fa fa-caret-down' />
                                    </a>
                                )}
                                items={sortOptions.map(so => ({
                                    title: so.title,
                                    action: () => {
                                        if (!pref.sortOptions) {
                                            pref.sortOptions = {};
                                        }
                                        pref.sortOptions[preferencesKey] = so.title;
                                        if (!pref.sortDirections) {
                                            pref.sortDirections = {};
                                        }
                                        pref.sortDirections[preferencesKey] = 'asc';
                                        services.viewPreferences.updatePreferences(pref);
                                        onPageChange(0);
                                    }
                                }))}
                            />
                        )}
                        <DropDownMenu
                            qeId={`paginate-items-per-page-${preferencesKey}`}
                            anchor={() => (
                                <a onMouseDown={() => document.body.click()}>
                                    Items per page: {pageSize === -1 ? 'all' : pageSize} <i className='fa fa-caret-down' />
                                </a>
                            )}
                            items={[5, 10, 15, 20, -1].map(count => ({
                                title: count === -1 ? 'all' : count.toString(),
                                action: () => {
                                    pref.pageSizes[preferencesKey] = count;
                                    services.viewPreferences.updatePreferences(pref);
                                }
                            }))}
                        />
                    </div>
                </div>
                {showHeader && header}
            </div>
        );
    }

    return (
        <React.Fragment>
            <div className='paginate'>{paginator()}</div>
            {sortedData.length === 0 && emptyState ? emptyState() : children(pageSize === -1 ? sortedData : sortedData.slice(pageSize * currentPage, pageSize * (currentPage + 1)))}
            <div className='paginate'>{pageCount > 1 && paginator()}</div>
        </React.Fragment>
    );
}
