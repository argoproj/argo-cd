import requests from './requests';

export class UserService {
    public login(username: string, password: string): Promise<{ token: string }> {
        return requests.post('/session').send({ username, password }).then((res) => ({token: res.body.token}));
    }

    public logout(): Promise<boolean> {
        return requests.delete('/session').then((res) => true);
    }
}
