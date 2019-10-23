import {AuthSettings} from '../models';
import requests from './requests';

export class AuthService {
    public settings(): Promise<AuthSettings> {
        return requests.get('/settings').then(res => res.body as AuthSettings);
    }
}
