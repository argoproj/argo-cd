import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {NavigationManager} from 'argo-ui';
import {createMemoryHistory} from 'history';

import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ResourcesTable} from './resources-table';

const preferences = services.viewPreferences;

beforeAll(() => preferences.init());
beforeEach(() => preferences.updatePreferences({sortOptions: undefined, sortDirections: undefined}));

function renderTable(onRender = jest.fn()) {
    const history = createMemoryHistory({initialEntries: ['/resources?page=3&kind=Pod']});
    render(
        <Context.Provider value={{navigation: new NavigationManager(history), history, baseHref: '/', popup: null, notifications: null}}>
            <React.Profiler id='resources-table' onRender={onRender}>
                <ResourcesTable resources={[]} />
            </React.Profiler>
        </Context.Provider>
    );
    return history;
}

const arrow = (column: string) => screen.getByTitle(column).querySelector('i.fa-sort-up, i.fa-sort-down')?.className;

test('header clicks toggle the current sort, start a new column ascending, and reset the page', () => {
    const history = renderTable();
    expect(arrow('NAME')).toContain('fa-sort-up');

    fireEvent.click(screen.getByTitle('NAME'));
    expect(arrow('NAME')).toContain('fa-sort-down');
    expect(new URLSearchParams(history.location.search).get('page')).toBe('0');
    expect(new URLSearchParams(history.location.search).get('kind')).toBe('Pod');
    expect(history.action).toBe('REPLACE');

    fireEvent.click(screen.getByTitle('NAMESPACE'));
    expect(arrow('NAME')).toBeUndefined();
    expect(arrow('NAMESPACE')).toContain('fa-sort-up');
    expect(preferences.getCurrentPreferences()).toMatchObject({sortOptions: {'resources-list': 'Namespace'}, sortDirections: {'resources-list': 'asc'}});
});

test('the arrow follows external sort changes without rerendering for other lists', () => {
    const sortOptions = {'resources-list': 'Status'};
    const sortDirections = {'resources-list': 'desc' as const};
    preferences.updatePreferences({sortOptions, sortDirections});
    const onRender = jest.fn();
    renderTable(onRender);
    expect(arrow('STATUS')).toContain('fa-sort-down');
    onRender.mockClear();

    act(() => preferences.updatePreferences({sortOptions: {...sortOptions, projects: 'Name'}, sortDirections: {...sortDirections, projects: 'desc'}}));
    expect(onRender).not.toHaveBeenCalled();

    act(() => {
        const pref = preferences.getCurrentPreferences();
        pref.sortDirections['resources-list'] = 'asc';
        preferences.updatePreferences(pref);
    });
    expect(arrow('STATUS')).toContain('fa-sort-up');
    fireEvent.click(screen.getByTitle('STATUS'));
    expect(preferences.getCurrentPreferences()).toMatchObject({
        sortOptions: {'resources-list': 'Status', 'projects': 'Name'},
        sortDirections: {'resources-list': 'desc', 'projects': 'desc'}
    });
});
