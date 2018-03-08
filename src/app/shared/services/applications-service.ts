import { Observable } from 'rxjs';

import * as models from '../models';
import requests from './requests';

export class ApplicationsService {
    public list(): Promise<models.Application[]> {
        return requests.get('/applications').then((res) => res.body as models.ApplicationList).then((list) => list.items || []);
    }

    public get(name: string): Promise<models.Application> {
        return requests.get(`/applications/${name}`).then((res) => res.body as models.Application);
    }

    public watch(query?: {name: string}): Observable<models.ApplicationWatchEvent> {
        let url = '/stream/applications';
        if (query) {
            url = `${url}?name=${query.name}`;
        }
        return requests.loadEventSource(url).repeat().retry().map((data) => JSON.parse(data).result as models.ApplicationWatchEvent);
    }

    public sync(name: string, revision: string): Promise<boolean> {
        return requests.post(`/applications/${name}/sync`).send({ revision }).then((res) => true);
    }
}
