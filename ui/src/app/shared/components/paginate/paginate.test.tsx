import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../services';
import {Paginate, PaginateProps, SortOption} from './paginate';

jest.mock('../../services', () => {
    const {ViewPreferencesService} = jest.requireActual('../../services/view-preferences-service');
    return {services: {viewPreferences: new ViewPreferencesService()}};
});

jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    DropDownMenu: ({anchor, items, qeId}: {anchor: () => React.ReactNode; items: {title: string; action: () => void}[]; qeId: string}) => (
        <div aria-label={qeId}>
            {anchor()}
            {items.map(item => (
                <button key={item.title} onClick={item.action}>
                    {item.title}
                </button>
            ))}
        </div>
    )
}));

const data = Array.from({length: 12}, (_, i) => i + 1);
const sortOptions: SortOption<number>[] = [{title: 'Number', compare: (a, b) => a - b, defaultDirection: 'desc'}];
const rows = (items: number[], virtual: boolean) => <p data-testid='rows'>{`${items.join(',')}|virtual=${virtual}`}</p>;

function paginator(props: Partial<PaginateProps<number>> = {}) {
    return (
        <Paginate page={0} onPageChange={jest.fn()} data={data} preferencesKey='test' {...props}>
            {props.children || rows}
        </Paginate>
    );
}

beforeAll(() => services.viewPreferences.init());
beforeEach(() => services.viewPreferences.updatePreferences({pageSizes: {}, sortOptions: {}, sortDirections: {}, theme: 'light'}));

test('renders the default page size synchronously without a loading state', () => {
    render(paginator());

    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5,6,7,8,9,10|virtual=false');
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
});

test.each([undefined, ''])('uses the default preference key when preferencesKey is %s', preferencesKey => {
    services.viewPreferences.updatePreferences({pageSizes: {default: 5}});
    render(paginator({preferencesKey}));

    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5|virtual=false');
});

test('follows page size changes, including in-place updates and switching preference keys', () => {
    services.viewPreferences.updatePreferences({pageSizes: {test: 5, other: 2}});
    const {rerender} = render(paginator());
    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5|virtual=false');

    act(() => {
        const pref = services.viewPreferences.getCurrentPreferences();
        pref.pageSizes.test = 2;
        services.viewPreferences.updatePreferences(pref);
    });
    expect(screen.getByTestId('rows')).toHaveTextContent('1,2|virtual=false');

    rerender(paginator({preferencesKey: 'other', page: 1}));
    expect(screen.getByTestId('rows')).toHaveTextContent('3,4|virtual=false');
});

test('follows sort selection and in-place direction changes without mutating the input', () => {
    render(paginator({sortOptions: [...sortOptions, {title: 'Reverse', compare: (a, b) => b - a}]}));
    expect(screen.getByTestId('rows')).toHaveTextContent('12,11,10,9,8,7,6,5,4,3|virtual=false');

    act(() => services.viewPreferences.updatePreferences({sortDirections: {test: 'asc'}}));
    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5,6,7,8,9,10|virtual=false');

    act(() => {
        const pref = services.viewPreferences.getCurrentPreferences();
        pref.sortDirections.test = 'desc';
        services.viewPreferences.updatePreferences(pref);
    });
    expect(screen.getByTestId('rows')).toHaveTextContent('12,11,10,9,8,7,6,5,4,3|virtual=false');
    act(() => services.viewPreferences.updatePreferences({sortOptions: {test: 'Reverse'}}));
    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5,6,7,8,9,10|virtual=false');
    expect(data).toEqual(Array.from({length: 12}, (_, i) => i + 1));
});

test('does not re-render for unrelated preferences or another paginator key', () => {
    const children = jest.fn(rows);
    render(paginator({children}));
    children.mockClear();

    act(() => services.viewPreferences.updatePreferences({theme: 'dark', pageSizes: {other: 5}, sortOptions: {other: 'Name'}, sortDirections: {other: 'desc'}}));

    expect(children).not.toHaveBeenCalled();
});

test('menu actions preserve current preferences and other keys, and sorting resets the page', () => {
    const onPageChange = jest.fn();
    services.viewPreferences.updatePreferences({pageSizes: {test: 5}, sortDirections: {test: 'desc'}});
    render(paginator({sortOptions, page: 1, onPageChange}));

    act(() => services.viewPreferences.updatePreferences({theme: 'dark', pageSizes: {test: 5, other: 20}, sortOptions: {other: 'Name'}, sortDirections: {test: 'desc', other: 'desc'}}));
    fireEvent.click(screen.getAllByRole('button', {name: '10'})[0]);
    fireEvent.click(screen.getAllByRole('button', {name: 'Number'})[0]);

    expect(services.viewPreferences.getCurrentPreferences()).toMatchObject({
        theme: 'dark',
        pageSizes: {test: 10, other: 20},
        sortOptions: {test: 'Number', other: 'Name'},
        sortDirections: {test: 'asc', other: 'desc'}
    });
    expect(JSON.parse(window.localStorage.getItem('view_preferences')).pageSizes).toEqual({test: 10, other: 20});
    expect(onPageChange).toHaveBeenCalledWith(0);
});

test('show all passes every row and enables virtual scrolling', () => {
    render(paginator());
    fireEvent.click(screen.getAllByRole('button', {name: 'all'})[0]);

    expect(screen.getByTestId('rows')).toHaveTextContent('1,2,3,4,5,6,7,8,9,10,11,12|virtual=true');
    expect(screen.queryByRole('button', {name: 'Next'})).not.toBeInTheDocument();
});

test('preserves navigation, clamps the displayed page when data shrinks, and renders the empty state', () => {
    services.viewPreferences.updatePreferences({pageSizes: {test: 5}});
    const onPageChange = jest.fn();
    const {rerender} = render(paginator({page: 1, onPageChange}));
    expect(screen.getByTestId('rows')).toHaveTextContent('6,7,8,9,10|virtual=false');

    fireEvent.click(screen.getAllByRole('button', {name: 'Next'})[0]);
    expect(onPageChange).toHaveBeenLastCalledWith(2);
    fireEvent.click(screen.getAllByRole('button', {name: 'Previous'})[0]);
    expect(onPageChange).toHaveBeenLastCalledWith(0);

    rerender(paginator({data: [1, 2], page: 2}));
    expect(screen.getByTestId('rows')).toHaveTextContent('1,2|virtual=false');

    rerender(paginator({data: [], emptyState: () => <p>No rows</p>}));
    expect(screen.getByText('No rows')).toBeInTheDocument();
    expect(screen.queryByTestId('rows')).not.toBeInTheDocument();
});

test('highlights the correct page until the user navigates, and resets when the highlight changes', () => {
    services.viewPreferences.updatePreferences({pageSizes: {test: 5}});
    const onPageChange = jest.fn();
    const props = {focusItemKey: '12', getItemKey: String, onPageChange};
    const {rerender} = render(paginator(props));
    expect(onPageChange).toHaveBeenLastCalledWith(2);

    rerender(paginator({...props, page: 2}));
    fireEvent.click(screen.getAllByRole('button', {name: 'Previous'})[0]);
    expect(onPageChange).toHaveBeenLastCalledWith(1);
    onPageChange.mockClear();

    rerender(paginator({...props, page: 1}));
    act(() => services.viewPreferences.updatePreferences({pageSizes: {test: 2}}));
    expect(onPageChange).not.toHaveBeenCalled();

    rerender(paginator({...props, page: 1, focusItemKey: '1'}));
    expect(onPageChange).toHaveBeenLastCalledWith(0);
});
