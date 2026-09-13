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
        const results = getAppFilterResults([appInNamespace('ns1'), appInNamespace('ns2')], prefWithFavorites(['ns1/guestbook']));
        expect(results.map(result => result.filterResult.favourite)).toEqual([true, false]);
    });

    test('a favorite stored before the list became namespace-qualified still matches', () => {
        const results = getAppFilterResults([appInNamespace('ns1'), appInNamespace('ns2')], prefWithFavorites(['guestbook']));
        expect(results.map(result => result.filterResult.favourite)).toEqual([true, true]);
    });
});
