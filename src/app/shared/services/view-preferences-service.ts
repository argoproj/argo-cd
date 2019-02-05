import { BehaviorSubject, Observable } from 'rxjs';

export interface AppDetailsPreferences { defaultTreeFilter: string[]; }
export type AppsListViewType = 'tiles' | 'list' | 'summary';
export interface AppsListPreferences {
    projectsFilter: string[];
    reposFilter: string[];
    syncFilter: string[];
    healthFilter: string[];
    page: number;
    view: AppsListViewType;
}

export interface ViewPreferences {
    version: number;
    appDetails: AppDetailsPreferences;
    appList: AppsListPreferences;
}

const VIEW_PREFERENCES_KEY = 'view_preferences';

const minVer = 1;

const DEFAULT_PREFERENCES = {
    version: 1,
    appDetails: {
        defaultTreeFilter: [
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
    },
    appList: {
        page: 0,
        view: 'tiles' as AppsListViewType,
        projectsFilter: new Array<string>(),
        reposFilter: new Array<string>(),
        syncFilter: new Array<string>(),
        healthFilter: new Array<string>(),
    },
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
        const nextPref = Object.assign({}, this.preferencesSubj.getValue(), change);
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
