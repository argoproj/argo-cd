import * as cookie from 'cookie';

import {AuthSettings} from '../models';
import requests from './requests';

export class AuthService {
    public settings(): Promise<AuthSettings> {
        return requests.get('/settings').then((res) => res.body as AuthSettings);
    }

    public loggedOut(): boolean {
        return this.token() === null;
    }

    public token(): string {
        return cookie.parse(document.cookie)['argocd.token'];
    }
}
