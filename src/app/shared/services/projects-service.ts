import requests from './requests';

import * as models from '../models';

export interface ProjectParams {
    name: string;
    description: string;
    sourceRepos: string[];
    destinations: models.ApplicationDestination[];
}

function paramsToProj(params: ProjectParams) {
    return {
        metadata: { name: params.name },
        spec: {
            description: params.description,
            sourceRepos: params.sourceRepos,
            destinations: params.destinations,
        },
    };
}

export class ProjectsService {
    public list(): Promise<models.Project[]> {
        return requests.get('/projects').then((res) => res.body as models.ProjectList).then((list) => list.items);
    }

    public get(name: string): Promise<models.Project> {
        return requests.get(`/projects/${name}`).then((res) => res.body as models.Project);
    }

    public delete(name: string): Promise<boolean> {
        return requests.delete(`/projects/${name}`).then(() => true);
    }

    public create(params: ProjectParams): Promise<models.Project> {
        return requests.post('/projects').send({project: paramsToProj(params)}).then((res) => res.body as models.Project);
    }

    public async update(params: ProjectParams): Promise<models.Project> {
        const proj = await this.get(params.name);
        const update = paramsToProj(params);
        return requests.put(`/projects/${params.name}`).send({project: {...proj, spec: update.spec }}).then((res) => res.body as models.Project);
    }
}
