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

    public createHTTPS({
        type,
        name,
        url,
        username,
        password,
        tlsClientCertData,
        tlsClientCertKey,
        insecure,
        enableLfs,
        proxy,
        noProxy,
        project,
        forceHttpBasicAuth,
        enableOCI
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
        proxy: string;
        noProxy: string;
        project?: string;
        forceHttpBasicAuth?: boolean;
        enableOCI: boolean;
    }): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({type, name, repo: url, username, password, tlsClientCertData, tlsClientCertKey, insecure, enableLfs, proxy, noProxy, project, forceHttpBasicAuth, enableOCI})
            .then(res => res.body as models.Repository);
    }

    public updateHTTPS({
        type,
        name,
        url,
        username,
        password,
        tlsClientCertData,
        tlsClientCertKey,
        insecure,
        enableLfs,
        proxy,
        noProxy,
        project,
        forceHttpBasicAuth,
        enableOCI
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
        proxy: string;
        noProxy: string;
        project?: string;
        forceHttpBasicAuth?: boolean;
        enableOCI: boolean;
    }): Promise<models.Repository> {
        return requests
            .put(`/repositories/${encodeURIComponent(url)}`)
            .send({type, name, repo: url, username, password, tlsClientCertData, tlsClientCertKey, insecure, enableLfs, proxy, noProxy, project, forceHttpBasicAuth, enableOCI})
            .then(res => res.body as models.Repository);
    }

    public createSSH({
        type,
        name,
        url,
        sshPrivateKey,
        insecure,
        enableLfs,
        proxy,
        noProxy,
        project
    }: {
        type: string;
        name: string;
        url: string;
        sshPrivateKey: string;
        insecure: boolean;
        enableLfs: boolean;
        proxy: string;
        noProxy: string;
        project?: string;
    }): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({type, name, repo: url, sshPrivateKey, insecure, enableLfs, proxy, noProxy, project})
            .then(res => res.body as models.Repository);
    }

    public createGitHubApp({
        type,
        name,
        url,
        githubAppPrivateKey,
        githubAppId,
        githubAppInstallationId,
        githubAppEnterpriseBaseURL,
        tlsClientCertData,
        tlsClientCertKey,
        insecure,
        enableLfs,
        proxy,
        noProxy,
        project
    }: {
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
    }): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type,
                name,
                repo: url,
                githubAppPrivateKey,
                githubAppId,
                githubAppInstallationId,
                githubAppEnterpriseBaseURL,
                tlsClientCertData,
                tlsClientCertKey,
                insecure,
                enableLfs,
                proxy,
                noProxy,
                project
            })
            .then(res => res.body as models.Repository);
    }

    public createGoogleCloudSource({
        type,
        name,
        url,
        gcpServiceAccountKey,
        proxy,
        noProxy,
        project
    }: {
        type: string;
        name: string;
        url: string;
        gcpServiceAccountKey: string;
        proxy: string;
        noProxy: string;
        project?: string;
    }): Promise<models.Repository> {
        return requests
            .post('/repositories')
            .send({
                type,
                name,
                repo: url,
                gcpServiceAccountKey,
                proxy,
                noProxy,
                project
            })
            .then(res => res.body as models.Repository);
    }

    public delete(url: string, project: string): Promise<models.Repository> {
        return requests
            .delete(`/repositories/${encodeURIComponent(url)}?appProject=${project}`)
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

    public appDetails(source: models.ApplicationSource, appName: string, appProject: string, sourceIndex: number, versionId: number): Promise<models.RepoAppDetails> {
        return requests
            .post(`/repositories/${encodeURIComponent(source.repoURL)}/appdetails`)
            .send({source, appName, appProject, sourceIndex, versionId})
            .then(res => res.body as models.RepoAppDetails);
    }
}
