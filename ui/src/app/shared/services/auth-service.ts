import {AuthSettings, Plugin} from '../models';
import requests from './requests';

export class AuthService {
    private settingsPromise: Promise<AuthSettings> = null;

    public clearCache(): void {
        this.settingsPromise = null;
    }

    public settings(): Promise<AuthSettings> {
        if (!this.settingsPromise) {
            this.settingsPromise = requests
                .get('/settings')
                .then(res => res.body as AuthSettings)
                .catch(err => {
                    this.settingsPromise = null;
                    throw err;
                });
        }
        return this.settingsPromise;
    }

    public plugins(): Promise<Plugin[]> {
        return requests.get('/settings/plugins').then(res => (res.body.plugins || []) as Plugin[]);
    }
}
