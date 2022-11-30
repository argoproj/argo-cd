import * as models from '../models';
import requests from './requests';

export class ClustersService {
    public list(): Promise<models.Cluster[]> {
        return requests
            .get('/clusters')
            .then(res => res.body as models.ClusterList)
            .then(list => list.items || []);
    }

    public getByName(name: string): Promise<models.Cluster> {
        const requestUrl = `/clusters/${encodeURIComponent(name)}?id.type=name`;
        return requests.get(requestUrl).then(res => res.body as models.Cluster);
    }

    public updateByName(cluster: models.Cluster, ...paths: string[]): Promise<models.Cluster> {
        return requests
            .put(`/clusters/${encodeURIComponent(cluster.name)}`)
            .query({updatedFields: paths})
            .send(cluster)
            .then(res => res.body as models.Cluster);
    }

    public invalidateCacheByName(name: string): Promise<models.Cluster> {
        return requests
            .post(`/clusters/${encodeURIComponent(name)}/invalidate-cache`)
            .send({})
            .then(res => res.body as models.Cluster);
    }

    public deleteByName(name: string): Promise<models.Cluster> {
        return requests
            .delete(`/clusters/${encodeURIComponent(name)}`)
            .send()
            .then(res => res.body as models.Cluster);
    }
}
