import { getAppFilterResults, getAutoSyncStatus } from './applications-filter';
import { Application, SyncPolicy } from '../../../shared/models';
import { AppsListPreferences } from '../../../shared/services';

const AUTO_SYNC_ENABLED = 'Enabled';
const AUTO_SYNC_DISABLED = 'Disabled';

test('automated.enabled is true, return to `Enabled`.', () => {
    const syncPolicy = {
        automated: {
            enabled: true,
            prune: false,
            selfHeal: false
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_ENABLED);
});

test('automated.enabled is undefined, return to `Enabled`.', () => {
    const syncPolicy = {
        automated: {}
    } as unknown as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_ENABLED);
});

test('automated.enabled is false, return to `Disabled`.', () => {
    const syncPolicy = {
        automated: {
            enabled: false,
            prune: false,
            selfHeal: false
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_DISABLED);
});

test('syncPolicy is nil, return to `Disabled`', () => {
    expect(getAutoSyncStatus(undefined)).toBe(AUTO_SYNC_DISABLED);
});

test('automated is nil, return to `Disabled`.', () => {
    const syncPolicy = {} as SyncPolicy;
    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_DISABLED);
});

describe('favorites filter', () => {
    // The same application name in two namespaces, as possible with apps-in-any-namespace.
    const appInNamespace = (namespace: string) =>
        ({
            metadata: {name: 'guestbook', namespace},
            spec: {destination: {}, source: {}},
            status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}}
        }) as unknown as Application;

    const prefWithFavorites = (favoritesAppList: string[]) =>
        ({
            syncFilter: [],
            autoSyncFilter: [],
            healthFilter: [],
            namespacesFilter: [],
            clustersFilter: [],
            reposFilter: [],
            targetRevisionFilter: [],
            labelsFilter: [],
            annotationsFilter: [],
            operationFilter: [],
            showFavorites: true,
            favoritesAppList
        }) as unknown as AppsListPreferences;

    test('a namespace qualified favorite does not match the same name in another namespace', () => {
        const results = getAppFilterResults([appInNamespace('ns1'), appInNamespace('ns2')], prefWithFavorites(['ns1/guestbook']), false);
        expect(results.map(result => result.filterResult.favourite)).toEqual([true, false]);
    });

    test('a favorite stored before the list became namespace-qualified still matches', () => {
        const results = getAppFilterResults([appInNamespace('ns1'), appInNamespace('ns2')], prefWithFavorites(['guestbook']), false);
        expect(results.map(result => result.filterResult.favourite)).toEqual([true, true]);
    });
});

describe('sync source filter', () => {
    const hydratorApp = (hydrateTo: boolean, hydratedDrySHA?: string, syncedDrySHA?: string) =>
        ({
            metadata: {name: 'guestbook', namespace: 'argocd'},
            spec: {
                destination: {},
                sourceHydrator: {drySource: {}, syncSource: {targetBranch: 'env/dev'}, ...(hydrateTo && {hydrateTo: {targetBranch: 'env/dev-next'}})}
            },
            status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}, sourceHydrator: {syncedDrySHA, lastSuccessfulOperation: {hydratedDrySHA}}}
        }) as unknown as Application;

    const apps = [hydratorApp(true, 'new', 'old'), hydratorApp(true, 'new', 'new'), hydratorApp(true, undefined, 'old'), hydratorApp(false, 'new', 'old')];

    const prefWithSyncSource = (syncSourceFilter: string[]) =>
        ({
            syncFilter: [],
            autoSyncFilter: [],
            healthFilter: [],
            namespacesFilter: [],
            clustersFilter: [],
            reposFilter: [],
            targetRevisionFilter: [],
            labelsFilter: [],
            annotationsFilter: [],
            operationFilter: [],
            syncSourceFilter
        }) as unknown as AppsListPreferences;

    const syncSourceResults = (syncSourceFilter: string[], hydratorEnabled = true) =>
        getAppFilterResults(apps, prefWithSyncSource(syncSourceFilter), hydratorEnabled).map(result => result.filterResult.syncSource);

    test('matches only apps whose synced dry SHA differs from the last hydrated one', () => {
        expect(syncSourceResults(['Behind'])).toEqual([true, false, false, false]);
    });

    test('matches apps whose dry SHAs are equal', () => {
        expect(syncSourceResults(['Up to date'])).toEqual([false, true, false, false]);
    });

    test('matches apps without hydrateTo or with an unknown dry SHA', () => {
        expect(syncSourceResults(['None'])).toEqual([false, false, true, true]);
    });

    test('is ignored when the hydrator is disabled', () => {
        expect(syncSourceResults(['Behind'], false)).toEqual([true, true, true, true]);
    });
});
