import {AuthSettings, Plugin} from '../models';
import requests from './requests';

export class AuthService {
    public settings(): Promise<AuthSettings> {
        return requests.get('/settings').then(res => res.body as AuthSettings);
    }

    public plugins(): Promise<Plugin[]> {
        return requests.get('/settings/plugins').then(res => (res.body.plugins || []) as Plugin[]);
    }
}
