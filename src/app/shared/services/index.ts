import { ApplicationsService } from './applications-service';
import { AuthService } from './auth-service';
import { RepositoriesService } from './repo-service';
import { UserService } from './user-service';

export interface Services {
    applications: ApplicationsService;
    userService: UserService;
    authService: AuthService;
    reposService: RepositoriesService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    userService: new UserService(),
    authService: new AuthService(),
    reposService: new RepositoriesService(),
};
