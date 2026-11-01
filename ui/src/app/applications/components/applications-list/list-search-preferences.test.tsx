import * as React from 'react';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import {NavigationManager} from 'argo-ui';
import {KeybindingProvider} from 'argo-ui/v2';

import {Context} from '../../../shared/context';
import {services, ViewPreferencesService} from '../../../shared/services';
import {ResourcesToolbar, ViewPref as ResourcesViewPref} from '../../../resources/components/resources-list/resources-list';
import {ApplicationSetsToolbar, ViewPref as ApplicationSetsViewPref} from './application-sets-list';
import {ApplicationsToolbar, ViewPref as ApplicationsViewPref} from './applications-list';
import {ViewTypeSwitcher} from './view-type-switcher';

jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    Tooltip: ({children}: {children: React.ReactNode}) => children
}));

jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: unknown) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

const pages = ['applications', 'applicationsets', 'resources'] as const;
type ListPage = (typeof pages)[number];
const searchKeys = {applications: 'appListSearch', applicationsets: 'appSetListSearch', resources: 'resourcesListSearch'};
const originalPreferences = services.viewPreferences;

function reloadPreferences() {
    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
}

function storedPreferences() {
    return JSON.parse(window.localStorage.getItem('view_preferences'));
}

beforeEach(() => {
    window.localStorage.clear();
    reloadPreferences();
});

afterEach(() => {
    services.viewPreferences = originalPreferences;
});

function renderList(page: ListPage, query = '') {
    const history = createMemoryHistory({initialEntries: ['/' + page + query]});
    const ctx = {history, navigation: new NavigationManager(history)} as React.ContextType<typeof Context>;
    let content: React.ReactNode;
    if (page === 'applications') {
        content = (
            <ApplicationsViewPref>
                {pref => (
                    <>
                        <ApplicationsToolbar applications={[]} pref={pref} ctx={ctx} healthBarPrefs={pref.statusBarView} />
                        <ViewTypeSwitcher pref={pref} ctx={ctx} />
                    </>
                )}
            </ApplicationsViewPref>
        );
    } else if (page === 'applicationsets') {
        content = (
            <ApplicationSetsViewPref>
                {pref => (
                    <>
                        <ApplicationSetsToolbar appSets={[]} pref={pref} ctx={ctx} healthBarPrefs={pref.statusBarView} />
                        <ViewTypeSwitcher pref={pref} ctx={ctx} />
                    </>
                )}
            </ApplicationSetsViewPref>
        );
    } else {
        content = <ResourcesViewPref>{pref => <ResourcesToolbar pref={pref} ctx={ctx} healthBarPrefs={pref.statusBarView} />}</ResourcesViewPref>;
    }
    return {
        history,
        ...render(
            <Context.Provider value={ctx}>
                <KeybindingProvider>{content}</KeybindingProvider>
            </Context.Provider>
        )
    };
}

test.each(pages)('%s search survives leaving the page and reloading preferences', async page => {
    const view = renderList(page);
    fireEvent.change(await screen.findByPlaceholderText(/^Search /), {target: {value: 'production'}});
    await waitFor(() => expect(new URLSearchParams(view.history.location.search).get('search')).toBe('production'));
    view.unmount();
    reloadPreferences();
    renderList(page);
    expect(await screen.findByDisplayValue('production')).toBeInTheDocument();
    expect(storedPreferences()[searchKeys[page]]).toBe('production');
});

test.each(pages)('%s uses URL search, including an explicit empty value, without overwriting the saved search', async page => {
    services.viewPreferences.updatePreferences({[searchKeys[page]]: 'saved'});
    const view = renderList(page, '?search=shared');
    expect(await screen.findByDisplayValue('shared')).toBeInTheDocument();
    act(() => view.history.push('/' + page + '?search='));
    expect(await screen.findByPlaceholderText(/^Search /)).toHaveValue('');
    expect(storedPreferences()[searchKeys[page]]).toBe('saved');
    act(() => view.history.goBack());
    expect(await screen.findByDisplayValue('shared')).toBeInTheDocument();
    view.unmount();
    reloadPreferences();
    renderList(page);
    expect(await screen.findByDisplayValue('saved')).toBeInTheDocument();
});

test.each(pages)('%s keeps an explicitly cleared search empty on the next visit', async page => {
    services.viewPreferences.updatePreferences({[searchKeys[page]]: 'saved'});
    const view = renderList(page, '?search=shared');
    fireEvent.change(await screen.findByDisplayValue('shared'), {target: {value: ''}});
    await waitFor(() => expect(new URLSearchParams(view.history.location.search).get('search')).toBe(''));
    view.unmount();
    reloadPreferences();
    renderList(page);
    expect(await screen.findByPlaceholderText(/^Search /)).toHaveValue('');
    expect(storedPreferences()[searchKeys[page]]).toBe('');
});

test('all three list searches stay independent across page changes and reloads', async () => {
    for (const page of pages) {
        const view = renderList(page);
        fireEvent.change(await screen.findByPlaceholderText(/^Search /), {target: {value: page + '-only'}});
        await waitFor(() => expect(storedPreferences()[searchKeys[page]]).toBe(page + '-only'));
        view.unmount();
    }
    reloadPreferences();
    for (const page of pages) {
        const view = renderList(page);
        expect(await screen.findByDisplayValue(page + '-only')).toBeInTheDocument();
        view.unmount();
    }
});

test.each(pages)('%s regex and health bar toggles do not save URL-only filters or transient fields', async page => {
    // Persist initialized defaults before comparing them with the rendered URL.
    services.viewPreferences.updatePreferences({});
    const key = page === 'resources' ? 'resourcesList' : 'appList';
    const saved = storedPreferences()[key];
    const view = renderList(page, '?health=Degraded&view=summary&search=shared&page=3');
    fireEvent.click(await screen.findByRole('button', {name: 'Toggle regex search'}));
    await waitFor(() => expect(storedPreferences()[key].searchRegex).toBe(true));
    const healthButton = view.container.querySelector('button.applications-list__accordion, button.resources-list__accordion');
    expect(healthButton).not.toBeNull();
    fireEvent.click(healthButton);
    await waitFor(() => expect(storedPreferences()[key].statusBarView.showHealthStatusBar).toBe(false));
    expect(storedPreferences()[key].healthFilter).toEqual(saved.healthFilter);
    expect(storedPreferences()[key].view).toBe(saved.view);
    expect(storedPreferences()[key]).not.toHaveProperty('search');
    expect(storedPreferences()[key]).not.toHaveProperty('page');
});

test.each(['applications', 'applicationsets'] as const)('%s view changes preserve saved filters instead of persisting URL overrides', async page => {
    services.viewPreferences.updatePreferences({});
    const saved = storedPreferences().appList;
    renderList(page, '?health=Degraded&search=shared&page=3');
    fireEvent.click(await screen.findByTitle('Summary'));
    await waitFor(() => expect(storedPreferences().appList.view).toBe('summary'));
    expect(storedPreferences().appList.healthFilter).toEqual(saved.healthFilter);
    expect(storedPreferences().appList).not.toHaveProperty('search');
    expect(storedPreferences().appList).not.toHaveProperty('page');
});
