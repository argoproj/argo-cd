import { ApplicationsService } from './applications-service';
import { AuthService } from './auth-service';
import { CertificatesService } from './cert-service';
import { ClustersService } from './clusters-service';
import { ProjectsService } from './projects-service';
import { RepositoriesService } from './repo-service';
import { UserService } from './user-service';
import { ViewPreferencesService } from './view-preferences-service';

export interface Services {
    applications: ApplicationsService;
    users: UserService;
    authService: AuthService;
    certs: CertificatesService;
    repos: RepositoriesService;
    clusters: ClustersService;
    projects: ProjectsService;
    viewPreferences: ViewPreferencesService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    authService: new AuthService(),
    clusters: new ClustersService(),
    users: new UserService(),
    certs: new CertificatesService(),
    repos: new RepositoriesService(),
    projects: new ProjectsService(),
    viewPreferences: new ViewPreferencesService(),
};

export { ProjectParams, ProjectRoleParams, CreateJWTTokenParams, DeleteJWTTokenParams, JWTTokenResponse } from './projects-service';
export * from './view-preferences-service';
