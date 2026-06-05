import {DataLoader, DropDownMenu} from 'argo-ui';

import * as React from 'react';
import {services} from '../../services';

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
}

export function Paginate<T>({page, onPageChange, children, data, emptyState, preferencesKey, header, showHeader, sortOptions}: PaginateProps<T>) {
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => {
                preferencesKey = preferencesKey || 'default';
                const pageSize = pref.pageSizes[preferencesKey] || 10;
                const sortOption = sortOptions ? (pref.sortOptions && pref.sortOptions[preferencesKey]) || sortOptions[0].title : '';
                const pageCount = pageSize === -1 ? 1 : Math.ceil(data.length / pageSize);
                if (pageCount <= page) {
                    page = pageCount - 1;
                }

                function paginator() {
                    const pageNumMinWidth = `${Math.max(2, String(pageCount).length)}ch`;
                    return (
                        <div style={{marginBottom: '0.5em'}}>
                            <div style={{display: 'flex', alignItems: 'center', marginBottom: '0.5em'}}>
                                {pageCount > 1 && <Paginator page={page} pageCount={pageCount} pageNumMinWidth={pageNumMinWidth} onPageChange={onPageChange} />}
                                <div className='paginate__size-menu'>
                                    {sortOptions && (
                                        <DropDownMenu
                                            qeId={`paginate-sort-${preferencesKey}`}
                                            anchor={() => (
                                                <>
                                                    <a>
                                                        Sort: {sortOption.toLowerCase()} <i className='fa fa-caret-down' />
                                                    </a>
                                                    &nbsp;
                                                </>
                                            )}
                                            items={sortOptions.map(so => ({
                                                title: so.title,
                                                action: () => {
                                                    // sortOptions might not be set in the browser storage
                                                    if (!pref.sortOptions) {
                                                        pref.sortOptions = {};
                                                    }
                                                    pref.sortOptions[preferencesKey] = so.title;
                                                    services.viewPreferences.updatePreferences(pref);
                                                }
                                            }))}
                                        />
                                    )}
                                    <DropDownMenu
                                        qeId={`paginate-items-per-page-${preferencesKey}`}
                                        anchor={() => (
                                            <a>
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
                if (sortOption) {
                    sortOptions
                        .filter(o => o.title === sortOption)
                        .forEach(so => {
                            data.sort(so.compare);
                        });
                }
                return (
                    <React.Fragment>
                        <div className='paginate'>{paginator()}</div>
                        {data.length === 0 && emptyState ? emptyState() : children(pageSize === -1 ? data : data.slice(pageSize * page, pageSize * (page + 1)))}
                        <div className='paginate'>{pageCount > 1 && paginator()}</div>
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
}
