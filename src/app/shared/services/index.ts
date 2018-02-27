import { ApplicationsService } from './applications-service';

export interface Services {
    applications: ApplicationsService;
}

export const services: Services = {
    applications: new ApplicationsService(),
};
