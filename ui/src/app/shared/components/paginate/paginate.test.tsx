import {render, screen} from '@testing-library/react';
import * as React from 'react';
import {of} from 'rxjs';

import {services} from '../../services';
import {Paginate} from './paginate';

jest.mock('../../services', () => ({
    services: {viewPreferences: {getPreferences: jest.fn(), updatePreferences: jest.fn()}}
}));

const renderPaginate = (data: string[], total?: number) => {
    jest.mocked(services.viewPreferences.getPreferences).mockReturnValue(of({pageSizes: {key: 2}} as any));
    const {container} = render(
        <Paginate page={0} total={total} preferencesKey='key' data={data} onPageChange={jest.fn()}>
            {rows => (
                <ul>
                    {rows.map(row => (
                        <li key={row}>{row}</li>
                    ))}
                </ul>
            )}
        </Paginate>
    );
    // The paginator renders above and below the rows.
    const pageLinks = container.querySelectorAll('.paginate__page-link').length / 2;
    return {pageLinks};
};

test('slices the data into pages on the client', () => {
    const {pageLinks} = renderPaginate(['a', 'b', 'c']);
    expect(screen.queryByText('a')).toBeInTheDocument();
    expect(screen.queryByText('b')).toBeInTheDocument();
    expect(screen.queryByText('c')).not.toBeInTheDocument();
    expect(pageLinks).toBe(2);
});

test('with a total, renders the data as the current page and counts pages from the total', () => {
    const {pageLinks} = renderPaginate(['c', 'd'], 6);
    expect(screen.queryByText('c')).toBeInTheDocument();
    expect(screen.queryByText('d')).toBeInTheDocument();
    expect(pageLinks).toBe(3);
});

test('with a total of zero, renders the empty state', () => {
    jest.mocked(services.viewPreferences.getPreferences).mockReturnValue(of({pageSizes: {key: 2}} as any));
    render(
        <Paginate page={0} total={0} preferencesKey='key' data={[]} onPageChange={jest.fn()} emptyState={() => <p>nothing here</p>}>
            {() => null}
        </Paginate>
    );
    expect(screen.getByText('nothing here')).toBeInTheDocument();
});
