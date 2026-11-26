import {act, render, screen} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import * as React from 'react';

import {Context} from '../context';
import {Query} from './query';

// Regression test for https://github.com/argoproj/argo-cd/issues/31076:
// a query-only navigation must re-render <Query> children so panels driven by
// the query string (project roles, sync windows) open without a page reload.
describe('Query', () => {
    it('re-renders when only the query string changes', () => {
        const history = createMemoryHistory({initialEntries: ['/settings/projects/default?tab=roles']});
        render(
            <Context.Provider value={{history} as any}>
                <Query>{q => <span data-testid='role'>{q.get('editRole') || 'none'}</span>}</Query>
            </Context.Provider>
        );
        expect(screen.getByTestId('role').textContent).toBe('none');

        act(() => history.push('/settings/projects/default?tab=roles&editRole=admin'));
        expect(screen.getByTestId('role').textContent).toBe('admin');

        act(() => history.push('/settings/projects/default?tab=roles'));
        expect(screen.getByTestId('role').textContent).toBe('none');
    });

    it('re-renders when a sync window is opened or closed', () => {
        const history = createMemoryHistory({initialEntries: ['/settings/projects/default?tab=windows']});
        render(
            <Context.Provider value={{history} as any}>
                <Query>{q => <span data-testid='window'>{q.get('editWindow') ?? (q.get('newWindow') ? 'new' : 'none')}</span>}</Query>
            </Context.Provider>
        );
        expect(screen.getByTestId('window').textContent).toBe('none');

        act(() => history.push('/settings/projects/default?tab=windows&editWindow=0'));
        expect(screen.getByTestId('window').textContent).toBe('0');

        act(() => history.push('/settings/projects/default?tab=windows&newWindow=true'));
        expect(screen.getByTestId('window').textContent).toBe('new');
    });
});
