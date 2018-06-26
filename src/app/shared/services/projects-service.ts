import requests from './requests';

import * as models from '../models';

export class ProjectsService {
    public list(): Promise<models.Project[]> {
        return requests.get('/projects').then((res) => res.body as models.ProjectList).then((list) => list.items);
    }
}
