import {showCreateFirstAppState} from './applications-list-empty-state';
import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';

const app = {metadata: {name: 'guestbook', namespace: 'ns1'}} as Application;

const pref = (overrides: Partial<AppsListPreferences> = {}) =>
    ({
        projectsFilter: [],
        labelsFilter: [],
        annotationsFilter: [],
        healthFilter: [],
        clustersFilter: [],
        namespacesFilter: [],
        reposFilter: [],
        targetRevisionFilter: [],
        syncFilter: [],
        autoSyncFilter: [],
        operationFilter: [],
        hydrationFilter: [],
        showFavorites: false,
        favoritesAppList: [],
        ...overrides
    }) as unknown as AppsListPreferences;

describe('showCreateFirstAppState', () => {
    it('is shown when there are no applications and no filters', () => {
        expect(showCreateFirstAppState([], pref())).toBe(true);
    });

    it('is not shown when there are applications', () => {
        expect(showCreateFirstAppState([app], pref())).toBe(false);
    });

    it('is not shown when the favorites filter is on, so the filter sidebar stays reachable', () => {
        // The favorites filter is applied server-side, so an empty list here means "nothing
        // favorited matched", not "no applications". Showing the create-first-application state
        // would hide the sidebar and leave the user unable to switch the filter back off.
        expect(showCreateFirstAppState([], pref({showFavorites: true}))).toBe(false);
        expect(showCreateFirstAppState([], pref({showFavorites: true, favoritesAppList: ['ns1/deleted-app']}))).toBe(false);
    });

    it('is not shown when a project or label filter could explain the empty list', () => {
        expect(showCreateFirstAppState([], pref({projectsFilter: ['default']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({labelsFilter: ['env=prod']}))).toBe(false);
    });

    it('is not shown when search could explain the empty list', () => {
        expect(showCreateFirstAppState([], pref(), 'no-such-app')).toBe(false);
        expect(showCreateFirstAppState([], pref(), '   ')).toBe(true);
    });

    it('is not shown when sidebar filters that clearFilters resets could explain the empty list', () => {
        expect(showCreateFirstAppState([], pref({healthFilter: ['Degraded']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({syncFilter: ['OutOfSync']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({reposFilter: ['https://example.com']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({clustersFilter: ['in-cluster']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({namespacesFilter: ['default']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({autoSyncFilter: ['Enabled']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({annotationsFilter: ['team=platform']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({operationFilter: ['Sync']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({hydrationFilter: ['Hydrated']}))).toBe(false);
        expect(showCreateFirstAppState([], pref({targetRevisionFilter: ['HEAD']}))).toBe(false);
    });
});
