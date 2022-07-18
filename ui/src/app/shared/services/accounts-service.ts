import {Account} from '../models';
import requests from './requests';

export class AccountsService {
    public list(): Promise<Account[]> {
        return requests.get('/account').then(res => (res.body.items || []) as Account[]);
    }

    public get(name: string): Promise<Account> {
        return requests.get(`/account/${name}`).then(res => res.body as Account);
    }

    public changePassword(name: string, currentPassword: string, newPassword: string): Promise<boolean> {
        return requests
            .put('/account/password')
            .send({currentPassword, name, newPassword})
            .then(res => res.status === 200);
    }

    public createToken(name: string, tokenId: string, expiresIn: number): Promise<string> {
        return requests
            .post(`/account/${name}/token`)
            .send({expiresIn, id: tokenId})
            .then(res => res.body.token as string);
    }

    public deleteToken(name: string, id: string): Promise<any> {
        return requests.delete(`/account/${name}/token/${id}`);
    }

    public canI(resource: string, action: string, subresource: string): Promise<boolean> {
        return requests.get(`/account/can-i/${resource}/${action}/${subresource}`).then(res => res.body.value === 'yes');
    }
}
