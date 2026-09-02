import {BehaviorSubject, Observable} from 'rxjs';

import {UserInfo} from '../models';
import {httpStatusOf} from '../utils';
import requests from './requests';

const permissionDenied = new BehaviorSubject<boolean>(false);

export class UserService {
    public get permissionDenied(): Observable<boolean> {
        return permissionDenied.asObservable();
    }

    public login(username: string, password: string): Promise<{token: string}> {
        return requests
            .post('/session')
            .send({username, password})
            .then(res => ({token: res.body.token}));
    }

    public logout(): Promise<boolean> {
        return requests.delete('/session').then(() => true);
    }

    public get(): Promise<UserInfo> {
        return requests
            .get('/session/userinfo')
            .then(res => res.body as UserInfo)
            .catch(err => {
                if (httpStatusOf(err) === 403) {
                    permissionDenied.next(true);
                }
                throw err;
            });
    }
}
