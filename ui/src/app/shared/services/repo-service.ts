import * as models from '../models';
import requests from './requests';

export interface HTTPSQuery {
    type: string;
    name: string;
    url: string;
    username: string;
    password: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    noProxy: string;
    project?: string;
    forceHttpBasicAuth?: boolean;
    enableOCI: boolean;
}

export interface SSHQuery {
    type: string;
    name: string;
    url: string;
    sshPrivateKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    noProxy: string;
    project?: string;
}

export interface GitHubAppQuery {
    type: string;
    name: string;
    url: string;
    githubAppPrivateKey: string;
    githubAppId: bigint;
    githubAppInstallationId: bigint;
    githubAppEnterpriseBaseURL: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    noProxy: string;
    project?: string;
}

export interface GoogleCloudSourceQuery {
    type: string;
    name: string;
    url: string;
    gcpServiceAccountKey: string;
    proxy: string;
    noProxy: string;
    project?: string;
}

export class RepositoriesService {
    public list(): Promise<models.Repository[]> {
        return requests
            .get(`/repositories`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public listWrite(): Promise<models.Repository[]> {
        return requests
            .get(`/write-repositories`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public listNoCache(): Promise<models.Repository[]> {
        return requests
            .get(`/repositories?forceRefresh=true`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public listWriteNoCache(): Promise<models.Repository[]> {
        return requests
            .get(`/write-repositories?forceRefresh=true`)
            .then(res => res.body as models.RepositoryList)
            .then(list => list.items || []);
    }

    public createHTTPS(q: HTTPSQuery): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                username: q.username,
                password: q.password,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project,
                forceHttpBasicAuth: q.forceHttpBasicAuth,
                enableOCI: q.enableOCI
            })
            .then(res => res.body as models.Repository);
    }

    public createHTTPSWrite(q: HTTPSQuery): Promise<models.Repository> {
        return requests
            .post('/write-repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                username: q.username,
                password: q.password,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project,
                forceHttpBasicAuth: q.forceHttpBasicAuth,
                enableOCI: q.enableOCI
            })
            .then(res => res.body as models.Repository);
    }

    public updateHTTPS(q: HTTPSQuery): Promise<models.Repository> {
        return requests
            .put(`/repositories/${encodeURIComponent(q.url)}`)
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                username: q.username,
                password: q.password,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project,
                forceHttpBasicAuth: q.forceHttpBasicAuth,
                enableOCI: q.enableOCI
            })
            .then(res => res.body as models.Repository);
    }

    public updateHTTPSWrite(q: HTTPSQuery): Promise<models.Repository> {
        return requests
            .put(`/write-repositories/${encodeURIComponent(q.url)}`)
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                username: q.username,
                password: q.password,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project,
                forceHttpBasicAuth: q.forceHttpBasicAuth,
                enableOCI: q.enableOCI
            })
            .then(res => res.body as models.Repository);
    }

    public createSSH(q: SSHQuery): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                sshPrivateKey: q.sshPrivateKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public createSSHWrite(q: SSHQuery): Promise<models.Repository> {
        return requests
            .post('/write-repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                sshPrivateKey: q.sshPrivateKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public createGitHubApp(q: GitHubAppQuery): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                githubAppPrivateKey: q.githubAppPrivateKey,
                githubAppId: q.githubAppId,
                githubAppInstallationId: q.githubAppInstallationId,
                githubAppEnterpriseBaseURL: q.githubAppEnterpriseBaseURL,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public createGitHubAppWrite(q: GitHubAppQuery): Promise<models.Repository> {
        return requests
            .post('/write-repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                githubAppPrivateKey: q.githubAppPrivateKey,
                githubAppId: q.githubAppId,
                githubAppInstallationId: q.githubAppInstallationId,
                githubAppEnterpriseBaseURL: q.githubAppEnterpriseBaseURL,
                tlsClientCertData: q.tlsClientCertData,
                tlsClientCertKey: q.tlsClientCertKey,
                insecure: q.insecure,
                enableLfs: q.enableLfs,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public createGoogleCloudSource(q: GoogleCloudSourceQuery): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                gcpServiceAccountKey: q.gcpServiceAccountKey,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public createGoogleCloudSourceWrite(q: GoogleCloudSourceQuery): Promise<models.Repository> {
        return requests
            .post('/write-repositories')
            .send({
                type: q.type,
                name: q.name,
                repo: q.url,
                gcpServiceAccountKey: q.gcpServiceAccountKey,
                proxy: q.proxy,
                noProxy: q.noProxy,
                project: q.project
            })
            .then(res => res.body as models.Repository);
    }

    public delete(url: string, project: string): Promise<models.Repository> {
        return requests
            .delete(`/repositories/${encodeURIComponent(url)}?appProject=${project}`)
            .send()
            .then(res => res.body as models.Repository);
    }

    public deleteWrite(url: string, project: string): Promise<models.Repository> {
        return requests
            .delete(`/write-repositories/${encodeURIComponent(url)}?appProject=${project}`)
            .send()
            .then(res => res.body as models.Repository);
    }

    public async revisions(repo: string): Promise<models.RefsInfo> {
        return requests.get(`/repositories/${encodeURIComponent(repo)}/refs`).then(res => res.body as models.RefsInfo);
    }

    public apps(repo: string, revision: string, appName: string, appProject: string): Promise<models.AppInfo[]> {
        return requests
            .get(`/repositories/${encodeURIComponent(repo)}/apps`)
            .query({revision})
            .query({appName})
            .query({appProject})
            .then(res => (res.body.items as models.AppInfo[]) || []);
    }

    public charts(repo: string): Promise<models.HelmChart[]> {
        return requests.get(`/repositories/${encodeURIComponent(repo)}/helmcharts`).then(res => (res.body.items as models.HelmChart[]) || []);
    }

    public appDetails(source: models.ApplicationSource, appName: string, appProject: string, sourceIndex: number, versionId: number | null): Promise<models.RepoAppDetails> {
        return requests
            .post(`/repositories/${encodeURIComponent(source.repoURL)}/appdetails`)
            .send({source, appName, appProject, sourceIndex, versionId})
            .then(res => res.body as models.RepoAppDetails);
    }
}
