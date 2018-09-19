import { ApplicationsService } from './applications-service';
import { AuthService } from './auth-service';
import { ClustersService } from './clusters-service';
import { ProjectsService } from './projects-service';
import { RepositoriesService } from './repo-service';
import { UserService } from './user-service';
import { ViewPreferencesService } from './view-preferences-service';

export interface Services {
    applications: ApplicationsService;
    userService: UserService;
    authService: AuthService;
    reposService: RepositoriesService;
    clustersService: ClustersService;
    projects: ProjectsService;
    viewPreferences: ViewPreferencesService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    authService: new AuthService(),
    clustersService: new ClustersService(),
    userService: new UserService(),
    reposService: new RepositoriesService(),
    projects: new ProjectsService(),
    viewPreferences: new ViewPreferencesService(),
};

export { ProjectParams, ProjectRoleParams, CreateJWTTokenParams, DeleteJWTTokenParams, JWTTokenResponse } from './projects-service';
