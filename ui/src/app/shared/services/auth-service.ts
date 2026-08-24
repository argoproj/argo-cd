import {AuthSettings, HealthCheckItem, Plugin} from '../models';
import requests from './requests';

export class AuthService {
    public settings(): Promise<AuthSettings> {
        return requests.get('/settings').then(res => res.body as AuthSettings);
    }

    public plugins(): Promise<Plugin[]> {
        return requests.get('/settings/plugins').then(res => (res.body.plugins || []) as Plugin[]);
    }

    public healthChecks(): Promise<HealthCheckItem[]> {
        return requests.get('/settings/health-checks').then(res => (res.body.healthChecks || []) as HealthCheckItem[]);
    }
}
