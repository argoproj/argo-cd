import {UserInfo} from '../models';
import {AccountsService} from './accounts-service';
import {AuthService} from './auth-service';
import requests from './requests';

export class UserService {
    private accountsService?: AccountsService;
    private authService?: AuthService;

    public setServices(accountsService: AccountsService, authService: AuthService) {
        this.accountsService = accountsService;
        this.authService = authService;
    }

    private clearCaches() {
        if (this.accountsService) {
            this.accountsService.clearCache();
        }
        if (this.authService) {
            this.authService.clearCache();
        }
    }

    public login(username: string, password: string): Promise<{token: string}> {
        return requests
            .post('/session')
            .send({username, password})
            .then(res => {
                this.clearCaches();
                return {token: res.body.token};
            });
    }

    public logout(): Promise<boolean> {
        return requests.delete('/session').then(() => {
            this.clearCaches();
            return true;
        });
    }

    public get(): Promise<UserInfo> {
        return requests.get('/session/userinfo').then(res => res.body as UserInfo);
    }
}
