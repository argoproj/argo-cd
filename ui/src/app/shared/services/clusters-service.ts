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

    public delete(server: string): Promise<models.Cluster> {
        return requests
            .delete(`/clusters/${encodeURIComponent(server)}`)
            .send()
            .then(res => res.body as models.Cluster);
    }
}
