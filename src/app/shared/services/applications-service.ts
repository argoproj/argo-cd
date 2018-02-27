import * as models from '../models';
import requests from './requests';

export class ApplicationsService {
    public async list(): Promise<models.Application[]> {
        return requests.get('/applications').then((res) => res.body as models.ApplicationList).then((list) => list.items);
    }
}
