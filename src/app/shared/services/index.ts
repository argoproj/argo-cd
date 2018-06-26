import { ApplicationsService } from './applications-service';
import { AuthService } from './auth-service';
import { ClustersService } from './clusters-service';
import { ProjectsService } from './projects-service';
import { RepositoriesService } from './repo-service';
import { UserService } from './user-service';

export interface Services {
    applications: ApplicationsService;
    userService: UserService;
    authService: AuthService;
    reposService: RepositoriesService;
    clustersService: ClustersService;
    projects: ProjectsService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    authService: new AuthService(),
    clustersService: new ClustersService(),
    userService: new UserService(),
    reposService: new RepositoriesService(),
    projects: new ProjectsService(),
};
