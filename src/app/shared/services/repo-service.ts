import * as models from '../models';
import requests from './requests';

export interface ArgoApp { ksonnet?: models.KsonnetAppSpec; helm?: models.HelmAppSpec; }

export class RepositoriesService {
    public list(): Promise<models.Repository[]> {
        return requests.get('/repositories').then((res) => res.body as models.RepositoryList).then((list) => list.items || []);
    }

    public create({url, username, password}: {url: string, username: string, password: string}): Promise<models.Repository> {
        return requests.post('/repositories').send({ repo: url, username, password }).then((res) => res.body as models.Repository);
    }

    public delete(url: string): Promise<models.Repository> {
        return requests.delete(`/repositories/${encodeURIComponent(url)}`).send().then((res) => res.body as models.Repository);
    }

    public apps(repo: string): Promise<ArgoApp[]> {
        return requests.get(`/repositories/${encodeURIComponent(repo)}/apps`)
            .then((res) => {
                const body = res.body as { ksonnetApps: models.KsonnetAppSpec[], helmApps: models.HelmAppSpec[] };
                const ksonnet = (body.ksonnetApps || []).map((item) => ({ ksonnet: item, helm: null }));
                const helm = (body.helmApps || []).map((item) => ({ ksonnet: null, helm: item }));
                return ksonnet.concat(helm);
            });
    }
}
