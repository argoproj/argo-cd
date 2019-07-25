import { BehaviorSubject, Observable } from 'rxjs';

export type AppsDetailsViewType = 'tree' | 'network' | 'list';

export interface AppDetailsPreferences {
    resourceFilter: string[];
    view: AppsDetailsViewType;
    resourceView: 'manifest' | 'diff';
    hideDefaultedFields: boolean;
    inlineDiff: boolean;
    compactDiff: boolean;
}

export type AppsListViewType = 'tiles' | 'list' | 'summary';

export interface AppsListPreferences {
    projectsFilter: string[];
    reposFilter: string[];
    syncFilter: string[];
    healthFilter: string[];
    namespacesFilter: string[];
    clustersFilter: string[];
    view: AppsListViewType;
}

export interface ViewPreferences {
    version: number;
    appDetails: AppDetailsPreferences;
    appList: AppsListPreferences;
    pageSizes: {[key: string]: number};
}

const VIEW_PREFERENCES_KEY = 'view_preferences';

const minVer = 3;

const DEFAULT_PREFERENCES: ViewPreferences = {
    version: 1,
    appDetails: {
        view: 'tree',
        resourceFilter: [
            'kind:Deployment',
            'kind:Service',
            'kind:Pod',
            'kind:StatefulSet',
            'kind:Ingress',
            'kind:ConfigMap',
            'kind:Job',
            'kind:DaemonSet',
            'kind:Workflow',
        ],
        hideDefaultedFields: false,
        inlineDiff: false,
        compactDiff: false,
        resourceView: 'manifest',
    },
    appList: {
        view: 'tiles' as AppsListViewType,
        projectsFilter: new Array<string>(),
        namespacesFilter: new Array<string>(),
        clustersFilter: new Array<string>(),
        reposFilter: new Array<string>(),
        syncFilter: new Array<string>(),
        healthFilter: new Array<string>(),
    },
    pageSizes: {},
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
        const nextPref = Object.assign({}, this.preferencesSubj.getValue(), change, { version: minVer });
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
