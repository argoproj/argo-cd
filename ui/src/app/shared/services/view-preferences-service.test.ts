import {AppsListPreferences, ViewPreferences, ViewPreferencesService} from './view-preferences-service';
import {firstValueFrom} from 'rxjs';

test('normalizes filter arrays dropped by a narrower appList write', async () => {
    window.localStorage.clear();
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

test('restores search text and other list preferences after reloading', async () => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    const preferences = await firstValueFrom(service.getPreferences());
    service.updatePreferences({appList: {...preferences.appList, search: '^prod-', searchRegex: true, projectsFilter: ['production']}});

    const reloaded = new ViewPreferencesService();
    reloaded.init();
    const restored = await firstValueFrom(reloaded.getPreferences());
    expect(restored.appList.search).toBe('^prod-');
    expect(restored.appList.searchRegex).toBe(true);
    expect(restored.appList.projectsFilter).toEqual(['production']);
});

test.each([undefined, null, 42, {}])('initializes absent or invalid saved search text (%p)', async search => {
    window.localStorage.clear();
    window.localStorage.setItem('view_preferences', JSON.stringify({version: 5, appList: {search, projectsFilter: ['production']}}));
    const service = new ViewPreferencesService();
    service.init();

    const preferences = await firstValueFrom(service.getPreferences());
    expect(preferences.appList.search).toBe('');
    expect(preferences.appList.projectsFilter).toEqual(['production']);
});
