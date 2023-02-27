import requests from './requests';

import * as models from '../models';

export interface CreateJWTTokenParams {
    project: string;
    role: string;
    expiresIn: number;
}

export interface DeleteJWTTokenParams {
    project: string;
    role: string;
    iat: number;
}

export interface JWTTokenResponse {
    token: string;
}

export interface ProjectSyncWindowsParams {
    projName: string;
    id: number;
    window: models.SyncWindow;
    deleteWindow: boolean;
}

export interface ProjectRoleParams {
    projName: string;
    roleName: string;
    description: string;
    policies: string[] | string;
    jwtTokens: models.JwtToken[];
    groups: string[];
    deleteRole: boolean;
    expiresIn: string;
}

function paramsToProjRole(params: ProjectRoleParams): models.ProjectRole {
    let newPolicies = [] as string[];
    if (typeof params.policies === 'string') {
        if (params.policies !== '') {
            newPolicies = params.policies.split('\n');
        }
    } else {
        newPolicies = params.policies;
    }
    return {
        name: params.roleName,
        description: params.description,
        policies: newPolicies,
        groups: params.groups
    };
}

export class ProjectsService {
    public list(...fields: string[]): Promise<models.Project[]> {
        return requests
            .get('/projects')
            .query({fields: fields.join(',')})
            .then(res => res.body as models.ProjectList)
            .then(list => list.items || []);
    }

    public get(name: string): Promise<models.Project> {
        return requests.get(`/projects/${name}`).then(res => res.body as models.Project);
    }

    public getDetailed(name: string): Promise<models.DetailedProjectsResponse> {
        return requests.get(`/projects/${name}/detailed`).then(res => res.body as models.DetailedProjectsResponse);
    }

    public getGlobalProjects(name: string): Promise<models.Project[]> {
        return requests.get(`/projects/${name}/globalprojects`).then(res => res.body.items as models.Project[]);
    }

    public delete(name: string): Promise<boolean> {
        return requests.delete(`/projects/${name}`).then(() => true);
    }

    public create(project: models.Project): Promise<models.Project> {
        return requests
            .post('/projects')
            .send({project})
            .then(res => res.body as models.Project);
    }

    public async update(project: models.Project): Promise<models.Project> {
        return requests
            .put(`/projects/${project.metadata.name}`)
            .send({project})
            .then(res => res.body as models.Project);
    }

    public getSyncWindows(name: string): Promise<models.SyncWindowsState> {
        return requests
            .get(`/projects/${name}/syncwindows`)
            .query({name})
            .then(res => res.body as models.SyncWindowsState);
    }

    public async updateWindow(params: ProjectSyncWindowsParams): Promise<models.Project> {
        const proj = await this.get(params.projName);
        const updatedSpec = proj.spec;
        if (proj.spec.syncWindows === undefined) {
            updatedSpec.syncWindows = [];
        }
        if (params.id === undefined || !(params.id in proj.spec.syncWindows)) {
            updatedSpec.syncWindows = updatedSpec.syncWindows.concat(params.window);
        } else {
            if (params.deleteWindow) {
                updatedSpec.syncWindows.splice(params.id, 1);
            } else {
                updatedSpec.syncWindows[params.id] = params.window;
            }
        }

        return requests
            .put(`/projects/${params.projName}`)
            .send({project: {...proj, spec: updatedSpec}})
            .then(res => res.body as models.Project);
    }

    public async updateRole(params: ProjectRoleParams): Promise<models.Project> {
        const proj = await this.get(params.projName);
        const updatedRole = paramsToProjRole(params);
        let roleExist = false;
        if (proj.spec.roles === undefined) {
            proj.spec.roles = [];
        }
        const updatedSpec = proj.spec;

        for (let i = 0; i < proj.spec.roles.length; i++) {
            if (proj.spec.roles[i].name === params.roleName) {
                roleExist = true;
                if (params.deleteRole) {
                    updatedSpec.roles.splice(i, 1);
                    break;
                }
                updatedSpec.roles[i] = updatedRole;
            }
        }
        if (!roleExist) {
            if (updatedSpec.roles === undefined) {
                updatedSpec.roles = [];
            }
            updatedSpec.roles = updatedSpec.roles.concat(updatedRole);
        }
        return requests
            .put(`/projects/${params.projName}`)
            .send({project: {...proj, spec: updatedSpec}})
            .then(res => res.body as models.Project);
    }

    public async createJWTToken(params: CreateJWTTokenParams): Promise<JWTTokenResponse> {
        return requests
            .post(`/projects/${params.project}/roles/${params.role}/token`)
            .send(params)
            .then(res => res.body as JWTTokenResponse);
    }

    public async deleteJWTToken(params: DeleteJWTTokenParams): Promise<boolean> {
        return requests
            .delete(`/projects/${params.project}/roles/${params.role}/token/${params.iat}`)
            .send()
            .then(() => true);
    }

    public events(projectName: string): Promise<models.Event[]> {
        return requests
            .get(`/projects/${projectName}/events`)
            .send()
            .then(res => (res.body as models.EventList).items || []);
    }

    public getLinks(projectName: string): Promise<models.LinksResponse> {
        return requests
            .get(`/projects/${projectName}/links`)
            .send()
            .then(res => res.body as models.LinksResponse);
    }
}
