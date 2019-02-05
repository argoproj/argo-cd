import * as React from 'react';
import ReactPaginate from 'react-paginate';

require('./paginate.scss');

export interface PaginateProps<T> {
    page: number;
    onPageChange: (page: number) => any;
    pageLimit: number;
    children: (data: T[]) => React.ReactNode;
    data: T[];
    emptyState?: () => React.ReactNode;
}

export function Paginate<T>({page, onPageChange, pageLimit, children, data, emptyState}: PaginateProps<T>) {
    const pageCount = Math.ceil(data.length / pageLimit);
    if (pageCount <= page) {
        page = pageCount - 1;
    }
    return (
        <React.Fragment>
            <div className='paginate'>
            {pageCount > 1 && (
                <ReactPaginate forcePage={page} pageCount={pageCount} pageRangeDisplayed={5} marginPagesDisplayed={2} onPageChange={(item) => onPageChange(item.selected)} />
            )}
            </div>
            {data.length === 0 && emptyState ? emptyState() : children(data.slice(pageLimit * page, pageLimit * (page + 1)))}
            <div className='paginate'>
            {pageCount > 1 && (
                <ReactPaginate forcePage={page} pageCount={pageCount} pageRangeDisplayed={5} marginPagesDisplayed={2} onPageChange={(item) => onPageChange(item.selected)} />
            )}
            </div>
        </React.Fragment>
    );
}
