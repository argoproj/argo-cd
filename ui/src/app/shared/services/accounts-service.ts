import {Account} from '../models';
import requests from './requests';

export class AccountsService {
    public list(): Promise<Account[]> {
        return requests.get('/account').then(res => (res.body.items || []) as Account[]);
    }

    public get(name: string): Promise<Account> {
        return requests.get(`/account/${name}`).then(res => res.body as Account);
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
}
