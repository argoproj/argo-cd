import { BehaviorSubject, Observable } from 'rxjs';

export interface ViewPreferences {
    appDetails: { defaultKindFilter: string[] };
}

const VIEW_PREFERENCES_KEY = 'view_preferences';

const DEFAULT_PREFERENCES = {
    appDetails: { defaultKindFilter: ['Deployment', 'Service', 'Pod', 'StatefulSet', 'Ingress', 'ConfigMap', 'Job', 'DaemonSet', 'Workflow'] },
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
        } else {
            preferences = DEFAULT_PREFERENCES;
        }
        return preferences;
    }
}
