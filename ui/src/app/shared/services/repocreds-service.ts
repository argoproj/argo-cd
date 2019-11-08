import * as models from '../models';
import requests from './requests';

export class RepoCredsService {
    public list(): Promise<models.RepoCreds[]> {
        return requests
            .get('/repocreds')
            .then(res => res.body as models.RepoCredsList)
            .then(list => list.items || []);
    }

    public async createHTTPS({
        url,
        username,
        password,
        tlsClientCertData,
        tlsClientCertKey
    }: {
        url: string;
        username: string;
        password: string;
        tlsClientCertData: string;
        tlsClientCertKey: string;
    }): Promise<models.RepoCreds> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/repocreds')
            .set(requests.csrfHeaderName, csrfToken)
            .send({url, username, password, tlsClientCertData, tlsClientCertKey})
            .then(res => res.body as models.RepoCreds);
    }

    public async createSSH({url, sshPrivateKey}: {url: string; sshPrivateKey: string}): Promise<models.RepoCreds> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/repocreds')
            .set(requests.csrfHeaderName, csrfToken)
            .send({url, sshPrivateKey})
            .then(res => res.body as models.RepoCreds);
    }

    public async delete(url: string): Promise<models.RepoCreds> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete(`/repocreds/${encodeURIComponent(url)}`)
            .set(requests.csrfHeaderName, csrfToken)
            .send()
            .then(res => res.body as models.RepoCreds);
    }
}
