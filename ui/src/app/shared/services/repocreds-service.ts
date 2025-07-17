import * as models from '../models';
import requests from './requests';

export interface HTTPSCreds {
    url: string;
    username: string;
    password: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    proxy: string;
    noProxy: string;
}

export interface SSHCreds {
    url: string;
    sshPrivateKey: string;
}

export interface GitHubAppCreds {
    url: string;
    githubAppPrivateKey: string;
    githubAppId: bigint;
    githubAppInstallationId: bigint;
    githubAppEnterpriseBaseURL: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    proxy: string;
    noProxy: string;
}

export interface GoogleCloudSourceCreds {
    url: string;
    gcpServiceAccountKey: string;
}

export class RepoCredsService {
    public list(): Promise<models.RepoCreds[]> {
        return requests
            .get('/repocreds')
            .then(res => res.body as models.RepoCredsList)
            .then(list => list.items || []);
    }

    public listWrite(): Promise<models.RepoCreds[]> {
        return requests
            .get('/write-repocreds')
            .then(res => res.body as models.RepoCredsList)
            .then(list => list.items || []);
    }

    public createHTTPS(creds: HTTPSCreds): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createHTTPSWrite(creds: HTTPSCreds): Promise<models.RepoCreds> {
        return requests
            .post('/write-repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createSSH(creds: SSHCreds): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createSSHWrite(creds: SSHCreds): Promise<models.RepoCreds> {
        return requests
            .post('/write-repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createGitHubApp(creds: GitHubAppCreds): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createGitHubAppWrite(creds: GitHubAppCreds): Promise<models.RepoCreds> {
        return requests
            .post('/write-repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createGoogleCloudSource(creds: GoogleCloudSourceCreds): Promise<models.RepoCreds> {
        return requests
            .post('/repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public createGoogleCloudSourceWrite(creds: GoogleCloudSourceCreds): Promise<models.RepoCreds> {
        return requests
            .post('/write-repocreds')
            .send(creds)
            .then(res => res.body as models.RepoCreds);
    }

    public delete(url: string): Promise<models.RepoCreds> {
        return requests
            .delete(`/repocreds/${encodeURIComponent(url)}`)
            .send()
            .then(res => res.body as models.RepoCreds);
    }

    public deleteWrite(url: string): Promise<models.RepoCreds> {
        return requests
            .delete(`/write-repocreds/${encodeURIComponent(url)}`)
            .send()
            .then(res => res.body as models.RepoCreds);
    }
}
