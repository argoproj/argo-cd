import {AppsListPreferences, ViewPreferences, ViewPreferencesService} from './view-preferences-service';

const VIEW_PREFERENCES_KEY = 'view_preferences';

function readPreferences(service: ViewPreferencesService): ViewPreferences {
    let current: ViewPreferences;
    service
        .getPreferences()
        .subscribe(prefs => (current = prefs))
        .unsubscribe();
    return current;
}

beforeEach(() => {
    window.localStorage.clear();
});

test('normalizes filter arrays dropped by a narrower appList write', async () => {
    const service = new ViewPreferencesService();
    service.init();

    // the ApplicationSets page writes its narrower preferences into appList,
    // which lacks the Applications-only filter arrays such as hydrationFilter
    service.updatePreferences({
        appList: {
            labelsFilter: [],
            annotationsFilter: [],
            healthFilter: [],
            showFavorites: false,
            favoritesAppList: [],
            searchRegex: false
        } as unknown as AppsListPreferences
    });

    const prefs = await new Promise<ViewPreferences>(resolve => service.getPreferences().subscribe(resolve));
    expect(prefs.appList.hydrationFilter).toEqual([]);
    expect(prefs.appList.autoSyncFilter).toEqual([]);
    expect(prefs.appList.projectsFilter).toEqual([]);
});

test('appList.search defaults to an empty string when nothing is persisted', () => {
    const service = new ViewPreferencesService();
    service.init();

    expect(readPreferences(service).appList.search).toBe('');
});

test('appList.search is persisted and restored across reloads', () => {
    const service = new ViewPreferencesService();
    service.init();

    const appList = {...readPreferences(service).appList, search: 'guestbook'};
    service.updatePreferences({appList});

    expect(readPreferences(service).appList.search).toBe('guestbook');

    const reloaded = new ViewPreferencesService();
    reloaded.init();
    expect(readPreferences(reloaded).appList.search).toBe('guestbook');
});

test('appList.search is normalized to an empty string when missing from stored preferences', () => {
    window.localStorage.setItem(
        VIEW_PREFERENCES_KEY,
        JSON.stringify({
            version: 5,
            appList: {view: 'tiles'}
        })
    );

    const service = new ViewPreferencesService();
    service.init();

    expect(readPreferences(service).appList.search).toBe('');
});
