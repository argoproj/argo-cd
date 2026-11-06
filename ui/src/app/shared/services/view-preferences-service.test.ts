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

test('group by project is off by default and survives appList writes and reloads', async () => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    const read = (s: ViewPreferencesService) => new Promise<ViewPreferences>(resolve => s.getPreferences().subscribe(resolve));

    const defaults = await read(service);
    expect(defaults.groupAppsByProject).toBe(false);
    expect(defaults.collapsedProjectGroups).toEqual([]);

    service.updatePreferences({groupAppsByProject: true, collapsedProjectGroups: ['infra']});
    service.updatePreferences({appList: {labelsFilter: []} as unknown as AppsListPreferences});

    const reloaded = new ViewPreferencesService();
    reloaded.init();
    const prefs = await read(reloaded);
    expect(prefs.groupAppsByProject).toBe(true);
    expect(prefs.collapsedProjectGroups).toEqual(['infra']);
});
