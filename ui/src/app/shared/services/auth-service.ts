import { parse } from 'cookie';
import * as jwt from 'jwt-decode';

import { AuthSettings } from '../models';
import requests from './requests';

export class AuthService {
    public settings(): Promise<AuthSettings> {
        return requests.get('/settings').then((res) => res.body as AuthSettings);
    }

    public getCurrentUserId(): string {
        const cookies = parse(document.cookie);
        const token = cookies['argocd.token'];
        const user: any = token && jwt(token) || null;
        return (user && (user.email || user.sub)) || '';
    }
}
