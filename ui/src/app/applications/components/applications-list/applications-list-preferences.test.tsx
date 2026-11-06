import * as React from 'react';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import {NavigationManager} from 'argo-ui';
import {KeybindingProvider} from 'argo-ui/v2';

import {Context} from '../../../shared/context';
import {AppsListPreferences, services, ViewPreferencesService} from '../../../shared/services';
import {ApplicationsListSearchBar, ViewPref} from './applications-list';

jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: unknown) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

const originalPreferences = services.viewPreferences;

beforeEach(() => {
    window.localStorage.clear();
    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
});

afterEach(() => {
    services.viewPreferences = originalPreferences;
});

function getPreferences(): AppsListPreferences {
    let preferences: AppsListPreferences;
    services.viewPreferences
        .getPreferences()
        .subscribe(value => (preferences = value.appList))
        .unsubscribe();
    return preferences;
}

function renderList(initialSearch = '') {
    const history = createMemoryHistory({initialEntries: ['/applications' + initialSearch]});
    const ctx = {
        history,
        navigation: new NavigationManager(history)
    } as React.ContextType<typeof Context>;
    const page = (
        <Context.Provider value={ctx}>
            <KeybindingProvider>
                <ViewPref>{pref => <ApplicationsListSearchBar content={pref.search} searchRegex={pref.searchRegex} pref={pref} ctx={ctx} apps={[]} />}</ViewPref>
            </KeybindingProvider>
        </Context.Provider>
    );
    return {history, ...render(page)};
}

test('typing a search preserves it when returning without a search URL', async () => {
    const view = renderList();
    fireEvent.change(await screen.findByPlaceholderText('Search applications...'), {target: {value: 'production'}});
    await waitFor(() => expect(new URLSearchParams(view.history.location.search).get('search')).toBe('production'));
    view.unmount();

    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
    renderList();
    expect(await screen.findByDisplayValue('production')).toBeInTheDocument();
});

test('a shared search URL takes precedence over the saved search', async () => {
    services.viewPreferences.updatePreferences({appListSearch: 'saved'});
    renderList('?search=shared');
    expect(await screen.findByDisplayValue('shared')).toBeInTheDocument();
});

test('an explicitly empty search URL overrides a saved search', async () => {
    services.viewPreferences.updatePreferences({appListSearch: 'saved'});
    renderList('?search=');
    expect(await screen.findByPlaceholderText('Search applications...')).toHaveValue('');
});

test('clearing search remains cleared on the next visit', async () => {
    services.viewPreferences.updatePreferences({appListSearch: 'saved'});
    const view = renderList('?search=saved');
    fireEvent.change(await screen.findByDisplayValue('saved'), {target: {value: ''}});
    view.unmount();

    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
    renderList();
    expect(await screen.findByPlaceholderText('Search applications...')).toHaveValue('');
    expect(JSON.parse(window.localStorage.getItem('view_preferences')).appListSearch).toBe('');
});

test('regex searches survive returning to the list and retain the matching mode', async () => {
    services.viewPreferences.updatePreferences({appList: {...getPreferences(), searchRegex: true}});
    const view = renderList();
    fireEvent.change(await screen.findByPlaceholderText('Regex search (e.g. ^foo-.*-prod$)'), {target: {value: '^prod-'}});
    view.unmount();

    renderList();
    expect(await screen.findByDisplayValue('^prod-')).toBeInTheDocument();
    expect(getPreferences().searchRegex).toBe(true);
});

test('browser back navigation restores the previous URL search', async () => {
    const view = renderList('?search=first');
    expect(await screen.findByDisplayValue('first')).toBeInTheDocument();
    act(() => view.history.push('/applications?search=second'));
    expect(await screen.findByDisplayValue('second')).toBeInTheDocument();
    act(() => view.history.goBack());
    expect(await screen.findByDisplayValue('first')).toBeInTheDocument();
});

test.each([undefined, 'sets-only'])('ApplicationSets preference writes do not replace the Applications search (%p)', async search => {
    const view = renderList();
    fireEvent.change(await screen.findByPlaceholderText('Search applications...'), {target: {value: 'production'}});
    view.unmount();

    services.viewPreferences.updatePreferences({appList: {...getPreferences(), search, searchRegex: true} as AppsListPreferences});
    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
    renderList();
    expect(await screen.findByDisplayValue('production')).toBeInTheDocument();
});
