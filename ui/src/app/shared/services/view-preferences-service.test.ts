import {AppsListPreferences, ResourcesListPreferences, ViewPreferences, ViewPreferencesService} from './view-preferences-service';
import {firstValueFrom} from 'rxjs';

test.each(['appList', 'resourcesList'] as const)('%s ignores equivalent filter arrays from initial synchronization', async key => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    const initial = (await firstValueFrom(service.getPreferences()))[key];
    service.updatePreferences({[key]: {...initial, healthFilter: ['Healthy']}});
    const previous = {...initial, healthFilter: ['Degraded']};
    const change = {...previous, healthFilter: ['Degraded']};
    if (key === 'appList') {
        service.updateAppListPreferences(change, previous);
    } else {
        service.updateResourcesListPreferences(change, previous);
    }
    expect((await firstValueFrom(service.getPreferences()))[key].healthFilter).toEqual(['Healthy']);
});

test('equivalent empty arrays preserve saved filters while changed and reordered arrays persist', async () => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    service.updateAppListPreferences({healthFilter: ['Healthy']});
    service.updateAppListPreferences({healthFilter: []}, {healthFilter: []});
    expect((await firstValueFrom(service.getPreferences())).appList.healthFilter).toEqual(['Healthy']);
    service.updateAppListPreferences({healthFilter: []}, {healthFilter: ['Degraded']});
    expect((await firstValueFrom(service.getPreferences())).appList.healthFilter).toEqual([]);
    service.updateAppListPreferences({healthFilter: ['Healthy', 'Degraded']}, {healthFilter: []});
    service.updateAppListPreferences({healthFilter: ['Degraded', 'Healthy']}, {healthFilter: ['Healthy', 'Degraded']});
    expect((await firstValueFrom(service.getPreferences())).appList.healthFilter).toEqual(['Degraded', 'Healthy']);
});

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
    service.updatePreferences({appListSearch: '^prod-', appList: {...preferences.appList, searchRegex: true, projectsFilter: ['production']}});

    const reloaded = new ViewPreferencesService();
    reloaded.init();
    const restored = await firstValueFrom(reloaded.getPreferences());
    expect(restored.appListSearch).toBe('^prod-');
    expect(restored.appList.searchRegex).toBe(true);
    expect(restored.appList.projectsFilter).toEqual(['production']);
});

test.each([undefined, null, 42, {}])('initializes absent or invalid saved search text on all lists (%p)', async search => {
    window.localStorage.clear();
    window.localStorage.setItem(
        'view_preferences',
        JSON.stringify({version: 5, appListSearch: search, appSetListSearch: search, resourcesListSearch: search, appList: {projectsFilter: ['production']}})
    );
    const service = new ViewPreferencesService();
    service.init();

    const preferences = await firstValueFrom(service.getPreferences());
    expect(preferences.appListSearch).toBe('');
    expect(preferences.appSetListSearch).toBe('');
    expect(preferences.resourcesListSearch).toBe('');
    expect(preferences.appList.projectsFilter).toEqual(['production']);
});

test('filter edits persist only changed fields and preserve Applications-only preferences after an ApplicationSets write', async () => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    service.updateAppListPreferences({projectsFilter: ['saved-project'], healthFilter: ['Healthy'], searchRegex: true});
    const saved = (await firstValueFrom(service.getPreferences())).appList;
    const rendered = {...saved, healthFilter: ['Degraded'], search: 'url-only', page: 2};
    service.updateAppListPreferences({...rendered, labelsFilter: ['env=prod']}, rendered);
    const prefs = (await firstValueFrom(service.getPreferences())).appList;
    expect(prefs.labelsFilter).toEqual(['env=prod']);
    expect(prefs.healthFilter).toEqual(['Healthy']);
    expect(prefs.projectsFilter).toEqual(['saved-project']);
    expect(prefs.searchRegex).toBe(true);
    expect(prefs).not.toHaveProperty('page');
    expect(prefs).not.toHaveProperty('search');
});

test('resource filter edits and clear preserve unrelated URL overrides and rendering state', async () => {
    window.localStorage.clear();
    const service = new ViewPreferencesService();
    service.init();
    service.updateResourcesListPreferences({healthFilter: ['Healthy'], projectsFilter: ['saved-project']});
    const saved = (await firstValueFrom(service.getPreferences())).resourcesList;
    const rendered = {...saved, healthFilter: ['Degraded'], search: 'url-only', page: 2, sortTitle: 'Name'};
    service.updateResourcesListPreferences({...rendered, kindFilter: ['Pod']}, rendered);
    let prefs = (await firstValueFrom(service.getPreferences())).resourcesList;
    expect(prefs.kindFilter).toEqual(['Pod']);
    expect(prefs.healthFilter).toEqual(['Healthy']);
    expect(prefs.projectsFilter).toEqual(['saved-project']);
    expect(prefs).not.toHaveProperty('page');
    expect(prefs).not.toHaveProperty('sortTitle');
    expect(prefs).not.toHaveProperty('search');
    const nextRendered = {...rendered, kindFilter: ['Pod']};
    const cleared = {...nextRendered};
    ResourcesListPreferences.clearFilters(cleared);
    service.updateResourcesListPreferences(cleared, nextRendered);
    prefs = (await firstValueFrom(service.getPreferences())).resourcesList;
    expect(prefs.kindFilter).toEqual([]);
    expect(prefs.healthFilter).toEqual([]);
    expect(prefs.projectsFilter).toEqual([]);
});
