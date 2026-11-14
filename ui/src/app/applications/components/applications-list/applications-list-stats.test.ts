import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';
import {getAppFilterResults} from './applications-filter';
import {collectMetadataValues, computeAppsListFacets, computeAppsListStats} from './applications-list-stats';

const app = (name: string, opts: {sync?: string; health?: string; namespace?: string; server?: string; labels?: {[k: string]: string}; repo?: string; revision?: string} = {}) =>
    ({
        metadata: {name, namespace: 'argocd', labels: opts.labels},
        spec: {
            destination: {server: opts.server || 'https://kubernetes.default.svc', namespace: opts.namespace},
            source: {repoURL: opts.repo || 'https://github.com/argoproj/argocd-example-apps', targetRevision: opts.revision || 'HEAD'}
        },
        status: {sync: {status: opts.sync || 'Synced'}, health: {status: opts.health || 'Healthy'}}
    }) as unknown as Application;

const pref = (overrides: Partial<AppsListPreferences> = {}) =>
    ({
        syncFilter: [],
        autoSyncFilter: [],
        healthFilter: [],
        hydrationFilter: [],
        namespacesFilter: [],
        clustersFilter: [],
        reposFilter: [],
        targetRevisionFilter: [],
        labelsFilter: [],
        annotationsFilter: [],
        operationFilter: [],
        projectsFilter: [],
        showFavorites: false,
        favoritesAppList: [],
        ...overrides
    }) as AppsListPreferences;

const apps = [
    app('a', {sync: 'Synced', health: 'Healthy', namespace: 'ns1'}),
    app('b', {sync: 'OutOfSync', health: 'Healthy', namespace: 'ns1'}),
    app('c', {sync: 'OutOfSync', health: 'Degraded', namespace: 'ns2'}),
    app('d', {sync: 'Synced', health: 'Degraded', namespace: 'ns2', server: 'https://other.example'})
];

describe('computeAppsListFacets', () => {
    test('counts every app when no filter is set', () => {
        const facets = computeAppsListFacets(getAppFilterResults(apps, pref(), false));
        expect(facets.totalBySyncStatus).toEqual({Synced: 2, OutOfSync: 2});
        expect(facets.totalByHealthStatus).toEqual({Healthy: 2, Degraded: 2});
        expect(facets.totalByAutoSyncStatus).toEqual({Disabled: 4});
    });

    test("a group's counts ignore its own filter but honour the others", () => {
        const facets = computeAppsListFacets(getAppFilterResults(apps, pref({syncFilter: ['Synced']}), false));
        // Sync ignores the sync filter, so OutOfSync stays selectable with its real count.
        expect(facets.totalBySyncStatus).toEqual({Synced: 2, OutOfSync: 2});
        // Health only counts the apps the sync filter lets through.
        expect(facets.totalByHealthStatus).toEqual({Healthy: 1, Degraded: 1});
    });

    test('an app failing two filters counts nowhere', () => {
        const facets = computeAppsListFacets(getAppFilterResults(apps, pref({syncFilter: ['Synced'], healthFilter: ['Healthy']}), false));
        // c (OutOfSync, Degraded) fails both, so it shows up in neither group.
        expect(facets.totalBySyncStatus).toEqual({Synced: 1, OutOfSync: 1});
        expect(facets.totalByHealthStatus).toEqual({Healthy: 1, Degraded: 1});
    });

    test('option lists come from every app regardless of filters', () => {
        const facets = computeAppsListFacets(getAppFilterResults(apps, pref({namespacesFilter: ['ns1']}), false));
        expect(facets.namespaces).toEqual(['ns1', 'ns2']);
        expect(facets.destinations).toEqual([
            {server: 'https://kubernetes.default.svc', name: undefined},
            {server: 'https://other.example', name: undefined}
        ]);
        expect(facets.repos).toEqual(['https://github.com/argoproj/argocd-example-apps']);
        expect(facets.revisions).toEqual(['HEAD']);
    });

    test('option lists drop empty values', () => {
        const facets = computeAppsListFacets(getAppFilterResults([app('a'), app('b', {namespace: 'ns1'})], pref(), false));
        expect(facets.namespaces).toEqual(['ns1']);
    });
});

describe('computeAppsListStats', () => {
    test('counts the given apps without faceting', () => {
        const stats = computeAppsListStats(apps.slice(0, 3));
        expect(stats.total).toBe(3);
        expect(stats.totalBySyncStatus).toEqual({Synced: 1, OutOfSync: 2});
        expect(stats.totalByHealthStatus).toEqual({Healthy: 2, Degraded: 1});
        expect(stats.totalByHydrationStatus).toEqual({None: 3});
        expect(stats.namespaces).toEqual(['ns1', 'ns2']);
        expect(stats.destinations).toHaveLength(1);
    });

    test('keeps an unset namespace as its own value', () => {
        expect(computeAppsListStats([app('a'), app('b', {namespace: 'ns1'})]).namespaces).toEqual([undefined, 'ns1']);
    });
});

test('collectMetadataValues groups values by key in first-seen order', () => {
    const items = [app('a', {labels: {team: 'x', env: 'prod'}}), app('b', {labels: {team: 'y'}}), app('c', {labels: {team: 'x'}}), app('d')];
    expect(collectMetadataValues(items, 'labels')).toEqual([
        {key: 'team', values: ['x', 'y']},
        {key: 'env', values: ['prod']}
    ]);
});
