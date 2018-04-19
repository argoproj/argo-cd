import { ApplicationsService } from './applications-service';
import { UserService } from './user-service';

export interface Services {
    applications: ApplicationsService;
    userService: UserService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    userService: new UserService(),
};
