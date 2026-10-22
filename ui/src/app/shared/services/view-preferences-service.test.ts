import {AppsListPreferences, ViewPreferences, ViewPreferencesService} from './view-preferences-service';

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
