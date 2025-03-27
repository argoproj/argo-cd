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
        tlsClientCertKey,
        proxy,
        noProxy
    }: {
        url: string;
        username: string;
        password: string;
        tlsClientCertData: string;
        tlsClientCertKey: string;
        proxy: string;
        noProxy: string;
    }): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send({url, username, password, tlsClientCertData, tlsClientCertKey, proxy, noProxy})
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
        tlsClientCertKey,
        proxy,
        noProxy
    }: {
        url: string;
        githubAppPrivateKey: string;
        githubAppId: bigint;
        githubAppInstallationId: bigint;
        githubAppEnterpriseBaseURL: string;
        tlsClientCertData: string;
        tlsClientCertKey: string;
        proxy: string;
        noProxy: string;
    }): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send({url, githubAppPrivateKey, githubAppId, githubAppInstallationId, githubAppEnterpriseBaseURL, tlsClientCertData, tlsClientCertKey, proxy, noProxy})
            .then(res => res.body as models.RepoCreds);
    }

    public createGoogleCloudSource({url, gcpServiceAccountKey}: {url: string; gcpServiceAccountKey: string}): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send({url, gcpServiceAccountKey})
            .then(res => res.body as models.RepoCreds);
    }

    public delete(url: string): Promise<models.RepoCreds> {
        return requests
            .delete(`/repocreds/${encodeURIComponent(url)}`)
            .send()
            .then(res => res.body as models.RepoCreds);
    }
}
