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

test('appList.search defaults to an empty string when nothing is persisted', () => {
    const service = new ViewPreferencesService();
    service.init();

    expect(readPreferences(service).appList.search).toBe('');
});

test('appList.search is persisted and restored across reloads', () => {
    const service = new ViewPreferencesService();
    service.init();

    const appList = {...readPreferences(service).appList, search: 'guestbook'} as AppsListPreferences;
    service.updatePreferences({appList});

    expect(readPreferences(service).appList.search).toBe('guestbook');

    // A fresh service reads the same backing store, simulating a page reload.
    const reloaded = new ViewPreferencesService();
    reloaded.init();
    expect(readPreferences(reloaded).appList.search).toBe('guestbook');
});

test('appList.search is normalized to an empty string when missing from stored preferences', () => {
    const stored = {
        version: 5,
        appList: {view: 'tiles'}
    };
    window.localStorage.setItem(VIEW_PREFERENCES_KEY, JSON.stringify(stored));

    const service = new ViewPreferencesService();
    service.init();

    expect(readPreferences(service).appList.search).toBe('');
});
