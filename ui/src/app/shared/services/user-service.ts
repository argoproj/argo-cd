import {UserInfo} from '../models';
import requests from './requests';

export class UserService {
    public async login(username: string, password: string): Promise<{token: string}> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/session')
            .set(requests.csrfHeaderName, csrfToken)
            .send({username, password})
            .then(res => ({token: res.body.token}));
    }

    public async logout(): Promise<boolean> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete('/session')
            .set(requests.csrfHeaderName, csrfToken)
            .then(() => true);
    }

    public get(): Promise<UserInfo> {
        return requests.get('/session/userinfo').then(res => res.body as UserInfo);
    }
}
