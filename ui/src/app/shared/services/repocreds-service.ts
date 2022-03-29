import * as models from '../models';
import requests from './requests';

export class RepoCredsService {
    public list(): Promise<models.RepoCreds[]> {
        return requests
            .get('/repocreds')
            .then(res => res.body as models.RepoCredsList)
            .then(list => list.items || []);
    }

    public createHTTPS({
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
        return requests
            .post('/repocreds')
            .send({url, username, password, tlsClientCertData, tlsClientCertKey})
            .then(res => res.body as models.RepoCreds);
    }

    public createSSH({url, sshPrivateKey}: {url: string; sshPrivateKey: string}): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send({url, sshPrivateKey})
            .then(res => res.body as models.RepoCreds);
    }

    public createGitHubApp({
        url,
        githubAppPrivateKey,
        githubAppId,
        githubAppInstallationId,
        githubAppEnterpriseBaseURL,
        tlsClientCertData,
        tlsClientCertKey
    }: {
        url: string;
        githubAppPrivateKey: string;
        githubAppId: bigint;
        githubAppInstallationId: bigint;
        githubAppEnterpriseBaseURL: string;
        tlsClientCertData: string;
        tlsClientCertKey: string;
    }): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send({url, githubAppPrivateKey, githubAppId, githubAppInstallationId, githubAppEnterpriseBaseURL, tlsClientCertData, tlsClientCertKey})
            .then(res => res.body as models.RepoCreds);
    }

    public delete(url: string): Promise<models.RepoCreds> {
        return requests
            .delete(`/repocreds/${encodeURIComponent(url)}`)
            .send()
            .then(res => res.body as models.RepoCreds);
    }
}
