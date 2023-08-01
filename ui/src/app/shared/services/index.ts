import {AccountsService} from './accounts-service';
import {ApplicationsService} from './applications-service';
import {ApplicationSetsService} from './applicationsets-service';
import {AuthService} from './auth-service';
import {CertificatesService} from './cert-service';
import {ClustersService} from './clusters-service';
import {ExtensionsService} from './extensions-service';
import {GnuPGPublicKeyService} from './gpgkey-service';
import {NotificationService} from './notification-service';
import {ProjectsService} from './projects-service';
import {RepositoriesService} from './repo-service';
import {RepoCredsService} from './repocreds-service';
import {UserService} from './user-service';
import {VersionService} from './version-service';
import {ViewPreferencesService} from './view-preferences-service';
// import {ViewAppSetPreferencesService} from './view-preferences-service';
export interface Services {
    applications: ApplicationsService;
    applicationSets: ApplicationSetsService;
    users: UserService;
    authService: AuthService;
    certs: CertificatesService;
    repocreds: RepoCredsService;
    repos: RepositoriesService;
    clusters: ClustersService;
    projects: ProjectsService;
    viewPreferences: ViewPreferencesService;
    // viewAppSetPreferences: ViewAppSetPreferencesService;
    version: VersionService;
    accounts: AccountsService;
    gpgkeys: GnuPGPublicKeyService;
    extensions: ExtensionsService;
    notification: NotificationService;
}

export const services: Services = {
    applications: new ApplicationsService(),
    applicationSets: new ApplicationSetsService(),
    authService: new AuthService(),
    clusters: new ClustersService(),
    users: new UserService(),
    certs: new CertificatesService(),
    repos: new RepositoriesService(),
    repocreds: new RepoCredsService(),
    projects: new ProjectsService(),
    viewPreferences: new ViewPreferencesService(),
    // viewAppSetPreferences: new ViewAppSetPreferencesService(),
    version: new VersionService(),
    accounts: new AccountsService(),
    gpgkeys: new GnuPGPublicKeyService(),
    extensions: new ExtensionsService(),
    notification: new NotificationService()
};

export * from './projects-service';
export * from './view-preferences-service';
