import * as models from '../models';
import requests from './requests';

export class RepositoriesService {
    public list(): Promise<models.Repository[]> {
        return requests
            .get(`/repositories`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public listNoCache(): Promise<models.Repository[]> {
        return requests
            .get(`/repositories?forceRefresh=true`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public async createHTTPS({
        type,
        name,
        url,
        username,
        password,
        tlsClientCertData,
        tlsClientCertKey,
        insecure,
        enableLfs
    }: {
        type: string;
        name: string;
        url: string;
        username: string;
        password: string;
        tlsClientCertData: string;
        tlsClientCertKey: string;
        insecure: boolean;
        enableLfs: boolean;
    }): Promise<models.Repository> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/repositories')
            .set(requests.csrfHeaderName, csrfToken)
            .send({type, name, repo: url, username, password, tlsClientCertData, tlsClientCertKey, insecure, enableLfs})
            .then(res => res.body as models.Repository);
    }

    public async createSSH({
        type,
        name,
        url,
        sshPrivateKey,
        insecure,
        enableLfs
    }: {
        type: string;
        name: string;
        url: string;
        sshPrivateKey: string;
        insecure: boolean;
        enableLfs: boolean;
    }): Promise<models.Repository> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/repositories')
            .set(requests.csrfHeaderName, csrfToken)
            .send({type, name, repo: url, sshPrivateKey, insecure, enableLfs})
            .then(res => res.body as models.Repository);
    }

    public async delete(url: string): Promise<models.Repository> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete(`/repositories/${encodeURIComponent(url)}`)
            .set(requests.csrfHeaderName, csrfToken)
            .send()
            .then(res => res.body as models.Repository);
    }

    public apps(repo: string, revision: string): Promise<models.AppInfo[]> {
        return requests
            .get(`/repositories/${encodeURIComponent(repo)}/apps`)
            .query({revision})
            .then(res => (res.body.items as models.AppInfo[]) || []);
    }

    public charts(repo: string): Promise<models.HelmChart[]> {
        return requests.get(`/repositories/${encodeURIComponent(repo)}/helmcharts`).then(res => (res.body.items as models.HelmChart[]) || []);
    }

    public async appDetails(source: models.ApplicationSource): Promise<models.RepoAppDetails> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post(`/repositories/${encodeURIComponent(source.repoURL)}/appdetails`)
            .set(requests.csrfHeaderName, csrfToken)
            .send({source})
            .then(res => res.body as models.RepoAppDetails);
    }
}
