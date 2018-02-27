import * as models from '../models';
import requests from './requests';

export class ApplicationsService {
    public list(): Promise<models.Application[]> {
        return requests.get('/applications').then((res) => res.body as models.ApplicationList).then((list) => list.items);
    }

    public get(name: string): Promise<models.Application> {
        return requests.get(`/applications/${name}`).then((res) => res.body as models.Application);
    }
}
