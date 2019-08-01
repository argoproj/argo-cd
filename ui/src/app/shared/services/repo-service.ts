import * as models from '../models';
import requests from './requests';

export class RepositoriesService {
    public list(): Promise<models.Repository[]> {
        return requests.get('/repositories').then((res) => res.body as models.RepositoryList).then((list) => list.items || []);
    }

    public createHTTPS({url, username, password, tlsClientCertData, tlsClientCertKey, insecure, enableLfs}:
        {url: string, username: string, password: string, tlsClientCertData: string, tlsClientCertKey: string,
            insecure: boolean, enableLfs: boolean}): Promise<models.Repository> {
        return requests.post('/repositories').send({ repo: url, username, password, tlsClientCertData, tlsClientCertKey, insecure, enableLfs })
            .then((res) => res.body as models.Repository);
    }

    public createSSH({url, sshPrivateKey, insecure, enableLfs}:
        {url: string, sshPrivateKey: string, insecure: boolean, enableLfs: boolean}): Promise<models.Repository> {
        return requests.post('/repositories').send({ repo: url, sshPrivateKey, insecure, enableLfs }).then((res) => res.body as models.Repository);
    }

    public delete(url: string): Promise<models.Repository> {
        return requests.delete(`/repositories/${encodeURIComponent(url)}`).send().then((res) => res.body as models.Repository);
    }

    public apps(repo: string, revision: string): Promise<models.AppInfo[]> {
        return requests.get(`/repositories/${encodeURIComponent(repo)}/apps`).query({revision})
            .then((res) => res.body.items as models.AppInfo[] || []);
    }

    public appDetails(repo: string, path: string, revision: string, details?: {
        helm?: { valueFiles: string[] },
        ksonnet?: { environment: string },
    }): Promise<models.RepoAppDetails> {
        const query: any = {revision};
        (details && details.helm && details.helm.valueFiles || []).forEach((file) => {
            query['helm.valueFiles'] = file;
        });
        if (details && details.ksonnet) {
            query['ksonnet.environment'] = details.ksonnet.environment;
        }
        return requests.get(`/repositories/${encodeURIComponent(repo)}/apps/${encodeURIComponent(path)}`).query(query)
            .then((res) => res.body as models.RepoAppDetails);
    }
}
