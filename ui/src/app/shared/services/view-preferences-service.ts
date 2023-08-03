import * as deepMerge from 'deepmerge';
import {BehaviorSubject, Observable} from 'rxjs';

import {PodGroupType} from '../../applications/components/application-pod-view/pod-view';

export type AppsDetailsViewType = 'tree' | 'network' | 'list' | 'pods';

export enum AppsDetailsViewKey {
    Tree = 'tree',
    Network = 'network',
    List = 'list',
    Pods = 'pods'
}

// export type AppSetsDetailsViewType = 'tree' |  'list' ;

// export enum AppSetsDetailsViewKey {
//     Tree = 'tree',
//     List = 'list',
// }

export interface AbstractAppDetailsPreferences {
    resourceFilter: string[];
    darkMode: boolean;
    hideFilters: boolean;
    groupNodes?: boolean;
    zoom: number;
    view: any;

    resourceView: 'manifest' | 'diff' | 'desiredManifest';
    inlineDiff: boolean;
    compactDiff: boolean;
    hideManagedFields?: boolean;
    orphanedResources: boolean;
    podView: PodViewPreferences;
    followLogs: boolean;
    wrapLines: boolean;
    podGroupCount: number;
}

export interface AppDetailsPreferences extends AbstractAppDetailsPreferences {
    view: AppsDetailsViewType | string;
    // resourceView: 'manifest' | 'diff' | 'desiredManifest';
    // inlineDiff: boolean;
    // compactDiff: boolean;
    // hideManagedFields?: boolean;
    // orphanedResources: boolean;
    // podView: PodViewPreferences;
    // followLogs: boolean;
    // wrapLines: boolean;
    // podGroupCount: number;
}

// export interface AppSetDetailsPreferences extends AbstractAppDetailsPreferences {
//     view: AppSetsDetailsViewType | string;
// }

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

export abstract class AbstractAppsListPreferences {
    public static countEnabledFilters(pref: AbstractAppsListPreferences) {}

    public static clearFilters(pref: AppsListPreferences) {}

    public labelsFilter: string[];
    public projectsFilter: string[];
    public reposFilter: string[];
    public syncFilter: string[];
    public autoSyncFilter: string[];
    public healthFilter: string[];
    public namespacesFilter: string[];
    public clustersFilter: string[];
    public view: AppsListViewType;
    public hideFilters: boolean;
    public statusBarView: HealthStatusBarPreferences;
    public showFavorites: boolean;
    public favoritesAppList: string[];
}

export class AppsListPreferences extends AbstractAppsListPreferences {
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
        pref.autoSyncFilter = [];
        pref.showFavorites = false;
    }
}

// export class AppSetsListPreferences extends AbstractAppsListPreferences {
//     public static countEnabledFilters(pref: AppSetsListPreferences) {
//         return [pref.labelsFilter].reduce(
//             (count, filter) => {
//                 if (filter && filter.length > 0) {
//                     return count + 1;
//                 }
//                 return count;
//             },
//             0
//         );
//     }

//     public static clearFilters(pref: AppSetsListPreferences) {
//         pref.labelsFilter = [];
//         pref.showFavorites = false;
//     }
// }

export interface AbstractViewPreferences {
    version: number;
    pageSizes: {[key: string]: number};
    sortOptions?: {[key: string]: string};
    hideBannerContent: string;
    hideSidebar: boolean;
    position: string;
    theme: string;
    appDetails: AbstractAppDetailsPreferences;
    appList: AbstractAppsListPreferences;
}

export interface ViewPreferences extends AbstractViewPreferences {
    appDetails: AppDetailsPreferences;
    appList: AppsListPreferences;
}

// export interface ViewAppSetPreferences extends AbstractViewPreferences {
//     appDetails: AppSetDetailsPreferences;
//     appList: AppSetsListPreferences;
// }

const VIEW_PREFERENCES_KEY = 'view_preferences';
// const VIEW_APPSET_PREFERENCES_KEY = 'view_app_set_preferences';

const minVer = 5;

const DEFAULT_PREFERENCES: ViewPreferences = {
    version: 1,
    appDetails: {
        view: 'tree',
        hideFilters: false,
        resourceFilter: [],
        inlineDiff: false,
        compactDiff: false,
        hideManagedFields: true,
        resourceView: 'manifest',
        orphanedResources: false,
        podView: {
            sortMode: 'node',
            hideUnschedulable: true
        },
        darkMode: false,
        followLogs: false,
        wrapLines: false,
        zoom: 1.0,
        podGroupCount: 15.0
    },
    appList: {
        view: 'tiles' as AppsListViewType,
        labelsFilter: new Array<string>(),
        projectsFilter: new Array<string>(),
        namespacesFilter: new Array<string>(),
        clustersFilter: new Array<string>(),
        reposFilter: new Array<string>(),
        syncFilter: new Array<string>(),
        autoSyncFilter: new Array<string>(),
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
    hideSidebar: false,
    position: '',
    theme: 'light'
};


// const DEFAULT_APPSET_PREFERENCES: ViewAppSetPreferences = {
//     version: 1,
//     appDetails: {
//         view: 'tree',
//         hideFilters: false,
//         resourceFilter: [],
//         inlineDiff: false,
//         compactDiff: false,
//         hideManagedFields: true,
//         resourceView: 'manifest',
//         orphanedResources: false,
//         podView: {
//             sortMode: 'node',
//             hideUnschedulable: true
//         },
//         darkMode: false,
//         followLogs: false,
//         wrapLines: false,
//         zoom: 1.0,
//         podGroupCount: 15.0
//     },
//     appList: {
//         view: 'tiles' as AppsListViewType,
//         labelsFilter: new Array<string>(),
//         projectsFilter: new Array<string>(),
//         namespacesFilter: new Array<string>(),
//         clustersFilter: new Array<string>(),
//         reposFilter: new Array<string>(),
//         syncFilter: new Array<string>(),
//         autoSyncFilter: new Array<string>(),
//         healthFilter: new Array<string>(),
//         hideFilters: false,
//         showFavorites: false,
//         favoritesAppList: new Array<string>(),
//         statusBarView: {
//             showHealthStatusBar: true
//         }
//     },
//     pageSizes: {},
//     hideBannerContent: '',
//     hideSidebar: false,
//     position: '',
//     theme: 'light'
// };

export abstract class AbstractViewPreferencesService {
    protected preferencesSubj: BehaviorSubject<AbstractViewPreferences>;

    public init() {
        if (!this.preferencesSubj) {
            this.preferencesSubj = new BehaviorSubject(this.loadPreferences());
            window.addEventListener('storage', () => {
                this.preferencesSubj.next(this.loadPreferences());
            });
        }
    }

    public getPreferences(): Observable<AbstractViewPreferences> {
        return this.preferencesSubj;
    }

    public abstract updatePreferences(change: Partial<AbstractViewPreferences>): void;

    protected abstract loadPreferences(): AbstractViewPreferences;
}


export class ViewPreferencesService extends AbstractViewPreferencesService {
    protected preferencesSubj: BehaviorSubject<ViewPreferences>;

    public updatePreferences(change: Partial<ViewPreferences>) {
        const nextPref = Object.assign({}, this.preferencesSubj.getValue(), change, {version: minVer});
        window.localStorage.setItem(VIEW_PREFERENCES_KEY, JSON.stringify(nextPref));
        this.preferencesSubj.next(nextPref);
    }

    protected loadPreferences(): AbstractViewPreferences {
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
        return deepMerge(DEFAULT_PREFERENCES, preferences);
    }
}

// export class ViewAppSetPreferencesService extends AbstractViewPreferencesService {
//     protected preferencesSubj: BehaviorSubject<ViewAppSetPreferences>;

//     public updatePreferences(change: Partial<AbstractViewPreferences>) {
//     }

//     protected loadPreferences(): AbstractViewPreferences {
//         let preferences: ViewAppSetPreferences;
//         const preferencesStr = window.localStorage.getItem(VIEW_APPSET_PREFERENCES_KEY);
//         if (preferencesStr) {
//             try {
//                 preferences = JSON.parse(preferencesStr);
//             } catch (e) {
//                 preferences = DEFAULT_APPSET_PREFERENCES;
//             }
//             if (!preferences.version || preferences.version < minVer) {
//                 preferences = DEFAULT_APPSET_PREFERENCES;
//             }
//         } else {
//             preferences = DEFAULT_APPSET_PREFERENCES;
//         }
//         return deepMerge(DEFAULT_APPSET_PREFERENCES, preferences);
//     }
// }
