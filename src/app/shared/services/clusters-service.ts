import * as models from '../models';
import requests from './requests';

export class ClustersService {
    public list(): Promise<models.Cluster[]> {
        return requests.get('/clusters').then((res) => res.body as models.ClusterList).then(((list) => list.items || []));
    }
}
