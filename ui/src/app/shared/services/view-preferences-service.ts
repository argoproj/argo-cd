import {BehaviorSubject, Observable} from 'rxjs';
import {PodGroupType} from '../../applications/components/application-pod-view/pod-view';

export type AppsDetailsViewType = 'tree' | 'network' | 'list' | 'pods';

export enum AppsDetailsViewKey {
    Tree = 'tree',
    Network = 'network',
    List = 'list',
    Pods = 'pods'
}

export interface AppDetailsPreferences {
    resourceFilter: string[];
    view: AppsDetailsViewType;
    resourceView: 'manifest' | 'diff' | 'desiredManifest';
    inlineDiff: boolean;
    compactDiff: boolean;
    hideManagedFields?: boolean;
    orphanedResources: boolean;
    podView: PodViewPreferences;
    darkMode: boolean;
    followLogs: boolean;
    hideFilters: boolean;
    wrapLines: boolean;
    groupNodes?: boolean;
    zoom?: number;
}

export interface PodViewPreferences {
    sortMode: PodGroupType;
    hideUnschedulable: boolean;
}

export interface HealthStatusBarPreferences {
    showHealthStatusBar: boolean;
}

export type AppsListViewType = 'tiles' | 'list' | 'summary';

export enum AppsListViewKey {
    List = 'list',
    Summary = 'summary',
    Tiles = 'tiles'
}

export class AppsListPreferences {
    public static countEnabledFilters(pref: AppsListPreferences) {
        return [pref.clustersFilter, pref.healthFilter, pref.labelsFilter, pref.namespacesFilter, pref.projectsFilter, pref.reposFilter, pref.syncFilter].reduce(
            (count, filter) => {
                if (filter && filter.length > 0) {
                    return count + 1;
                }
                return count;
            },
            0
        );
    }

    public static clearFilters(pref: AppsListPreferences) {
        pref.clustersFilter = [];
        pref.healthFilter = [];
        pref.labelsFilter = [];
        pref.namespacesFilter = [];
        pref.projectsFilter = [];
        pref.reposFilter = [];
        pref.syncFilter = [];
        pref.showFavorites = false;
    }

    public labelsFilter: string[];
    public projectsFilter: string[];
    public reposFilter: string[];
    public syncFilter: string[];
    public healthFilter: string[];
    public namespacesFilter: string[];
    public clustersFilter: string[];
    public view: AppsListViewType;
    public hideFilters: boolean;
    public statusBarView: HealthStatusBarPreferences;
    public showFavorites: boolean;
    public favoritesAppList: string[];
}

export interface ViewPreferences {
    version: number;
    appDetails: AppDetailsPreferences;
    appList: AppsListPreferences;
    pageSizes: {[key: string]: number};
    hideBannerContent: string;
    position: string;
}

const VIEW_PREFERENCES_KEY = 'view_preferences';

const minVer = 5;

const DEFAULT_PREFERENCES: ViewPreferences = {
    version: 1,
    appDetails: {
        view: 'tree',
        hideFilters: false,
        resourceFilter: [],
        inlineDiff: false,
        compactDiff: false,
        resourceView: 'manifest',
        orphanedResources: false,
        podView: {
            sortMode: 'node',
            hideUnschedulable: true
        },
        darkMode: false,
        followLogs: false,
        wrapLines: false,
        zoom: 1.0
    },
    appList: {
        view: 'tiles' as AppsListViewType,
        labelsFilter: new Array<string>(),
        projectsFilter: new Array<string>(),
        namespacesFilter: new Array<string>(),
        clustersFilter: new Array<string>(),
        reposFilter: new Array<string>(),
        syncFilter: new Array<string>(),
        healthFilter: new Array<string>(),
        hideFilters: false,
        showFavorites: false,
        favoritesAppList: new Array<string>(),
        statusBarView: {
            showHealthStatusBar: true
        }
    },
    pageSizes: {},
    hideBannerContent: '',
    position: ''
};

export class ViewPreferencesService {
    private preferencesSubj: BehaviorSubject<ViewPreferences>;

    public init() {
        if (!this.preferencesSubj) {
            this.preferencesSubj = new BehaviorSubject(this.loadPreferences());
            window.addEventListener('storage', () => {
                this.preferencesSubj.next(this.loadPreferences());
            });
        }
    }

    public getPreferences(): Observable<ViewPreferences> {
        return this.preferencesSubj;
    }

    public updatePreferences(change: Partial<ViewPreferences>) {
        const nextPref = Object.assign({}, this.preferencesSubj.getValue(), change, {version: minVer});
        window.localStorage.setItem(VIEW_PREFERENCES_KEY, JSON.stringify(nextPref));
        this.preferencesSubj.next(nextPref);
    }

    private loadPreferences(): ViewPreferences {
        let preferences: ViewPreferences;
        const preferencesStr = window.localStorage.getItem(VIEW_PREFERENCES_KEY);
        if (preferencesStr) {
            try {
                preferences = JSON.parse(preferencesStr);
            } catch (e) {
                preferences = DEFAULT_PREFERENCES;
            }
            if (!preferences.version || preferences.version < minVer) {
                preferences = DEFAULT_PREFERENCES;
            }
        } else {
            preferences = DEFAULT_PREFERENCES;
        }
        return Object.assign({}, DEFAULT_PREFERENCES, preferences);
    }
}
