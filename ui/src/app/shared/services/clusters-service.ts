import * as models from '../models';
import requests from './requests';

export class ClustersService {
    public list(): Promise<models.Cluster[]> {
        return requests
            .get('/clusters')
            .then(res => res.body as models.ClusterList)
            .then(list => list.items || []);
    }

    public get(url: string): Promise<models.Cluster> {
        return requests.get(`/clusters/${encodeURIComponent(url)}`).then(res => res.body as models.Cluster);
    }

    public update(cluster: models.Cluster): Promise<models.Cluster> {
        return requests
            .put(`/clusters/${encodeURIComponent(cluster.server)}`)
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
