import requests from './requests';

import * as models from '../models';

export interface ProjectParams {
    name: string;
    description: string;
    sourceRepos: string[];
    destinations: models.ApplicationDestination[];
    roles: models.ProjectRole[];
    clusterResourceWhitelist: models.GroupKind[];
    namespaceResourceBlacklist: models.GroupKind[];
}

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
        jwtTokens: params.jwtTokens,
        groups: params.groups,
    };
}

function paramsToProj(params: ProjectParams) {
    return {
        metadata: { name: params.name },
        spec: {
            description: params.description,
            sourceRepos: params.sourceRepos,
            destinations: params.destinations,
            roles: params.roles,
            clusterResourceWhitelist: params.clusterResourceWhitelist,
            namespaceResourceBlacklist: params.namespaceResourceBlacklist,
        },
    };
}

export class ProjectsService {
    public list(): Promise<models.Project[]> {
        return requests.get('/projects').then((res) => res.body as models.ProjectList).then((list) => list.items || []);
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
        return requests.put(`/projects/${params.projName}`).send({project: {...proj, spec: updatedSpec }}).then((res) => res.body as models.Project);
    }

    public async createJWTToken(params: CreateJWTTokenParams): Promise<JWTTokenResponse> {
        return requests.post(`/projects/${params.project}/roles/${params.role}/token`).send(params).then((res) => res.body as JWTTokenResponse);
    }

    public async deleteJWTToken(params: DeleteJWTTokenParams): Promise<boolean> {
        return requests.delete(`/projects/${params.project}/roles/${params.role}/token/${params.iat}`).send().then(() => true);
    }

    public events(projectName: string): Promise<models.Event[]> {
        return requests.get(`/projects/${projectName}/events`).send().then((res) => (res.body as models.EventList).items || []);
    }
}
