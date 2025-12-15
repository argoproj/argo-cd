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
        let idType = '';
        let idValue = '';
        if (name && url) {
            idType = 'url_name_escaped';
            idValue = encodeURIComponent(url + ',' + name);
        } else if (url) {
            idType = 'url';
            idValue = encodeURIComponent(url);
        } else if (name) {
            idType = 'name_escaped';
            idValue = encodeURIComponent(name);
        }
        const requestUrl = `/clusters/${idValue}?id.type=${idType}`;
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

    public delete(server: string, name: string): Promise<models.Cluster> {
        return requests
            .delete(`/clusters/${encodeURIComponent(server + ',' + name)}?id.type=url_name_escaped`)
            .send()
            .then(res => res.body as models.Cluster);
    }
}
