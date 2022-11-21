import * as models from '../models';
import requests from './requests';

export class ClustersService {
    public list(): Promise<models.Cluster[]> {
        return requests
            .get('/clusters')
            .then(res => res.body as models.ClusterList)
            .then(list => list.items || []);
    }

    public get(url: string, name: string): Promise<models.Cluster> {
        const requestUrl = `/clusters/${url ? encodeURIComponent(url) : encodeURIComponent(name)}?id.type=${url ? 'url' : 'name_escaped'}`;
        return requests.get(requestUrl).then(res => res.body as models.Cluster);
    }

    public update(cluster: models.Cluster, ...paths: string[]): Promise<models.Cluster> {
        return requests
            .put(`/clusters/${encodeURIComponent(cluster.server)}`)
            .query({updatedFields: paths})
            .send(cluster)
            .then(res => res.body as models.Cluster);
    }

    public invalidateCache(url: string): Promise<models.Cluster> {
        return requests
            .post(`/clusters/${encodeURIComponent(url)}/invalidate-cache`)
            .send({})
            .then(res => res.body as models.Cluster);
    }

    public delete(server: string): Promise<models.Cluster> {
        return requests
            .delete(`/clusters/${encodeURIComponent(server)}`)
            .send()
            .then(res => res.body as models.Cluster);
    }
}
