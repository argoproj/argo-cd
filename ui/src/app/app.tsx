import {DataLoader, NavigationManager, Notifications, NotificationsManager, PageContext, Popup, PopupManager, PopupProps} from 'argo-ui';
import {createBrowserHistory} from 'history';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Helmet} from 'react-helmet';
import {Redirect, Route, RouteComponentProps, Router, Switch} from 'react-router';
import applications from './applications';
import help from './help';
import login from './login';
import settings from './settings';
import {Layout} from './shared/components/layout/layout';
import {Page} from './shared/components/page/page';
import {VersionPanel} from './shared/components/version-info/version-info-panel';
import {AuthSettingsCtx, Provider} from './shared/context';
import {services} from './shared/services';
import requests from './shared/services/requests';
import {hashCode} from './shared/utils';
import {Banner} from './ui-banner/ui-banner';
import userInfo from './user-info';
import {AuthSettings} from './shared/models';
import {PKCEVerification} from './login/components/pkce-verify';
import {SystemLevelExtension} from './shared/services/extensions-service';

services.viewPreferences.init();
const bases = document.getElementsByTagName('base');
const base = bases.length > 0 ? bases[0].getAttribute('href') || '/' : '/';
export const history = createBrowserHistory({basename: base});
requests.setBaseHRef(base);

type Routes = {[path: string]: {component: React.ComponentType<RouteComponentProps<any>>; noLayout?: boolean}};

const routes: Routes = {
    '/login': {component: login.component as any, noLayout: true},
    '/applications': {component: applications.component},
    '/settings': {component: settings.component},
    '/user-info': {component: userInfo.component},
    '/help': {component: help.component},
    '/pkce/verify': {component: PKCEVerification, noLayout: true}
};

interface NavItem {
    title: string;
    tooltip?: string;
    path: string;
    iconClassName: string;
}

const navItems: NavItem[] = [
    {
        title: 'Applications',
        tooltip: 'Manage your applications, and diagnose health problems.',
        path: '/applications',
        iconClassName: 'argo-icon argo-icon-application'
    },
    {
        title: 'Settings',
        tooltip: 'Manage your repositories, projects, settings',
        path: '/settings',
        iconClassName: 'argo-icon argo-icon-settings'
    },
    {
        title: 'User Info',
        path: '/user-info',
        iconClassName: 'fa fa-user-circle'
    },
    {
        title: 'Documentation',
        tooltip: 'Read the documentation, and get help and assistance.',
        path: '/help',
        iconClassName: 'argo-icon argo-icon-docs'
    }
];

const versionLoader = services.version.version();

async function isExpiredSSO() {
    try {
        const {iss} = await services.users.get();
        const authSettings = await services.authService.settings();
        if (iss && iss !== 'argocd') {
            return ((authSettings.dexConfig && authSettings.dexConfig.connectors) || []).length > 0 || authSettings.oidcConfig;
        }
    } catch {
        return false;
    }
    return false;
}

requests.onError.subscribe(async err => {
    if (err.status === 401) {
        if (history.location.pathname.startsWith('/login')) {
            return;
        }

        const isSSO = await isExpiredSSO();
        // location might change after async method call, so we need to check again.
        if (history.location.pathname.startsWith('/login')) {
            return;
        }
        // Query for basehref and remove trailing /.
        // If basehref is the default `/` it will become an empty string.
        const basehref = document.querySelector('head > base').getAttribute('href').replace(/\/$/, '');
        if (isSSO) {
            window.location.href = `${basehref}/auth/login?return_url=${encodeURIComponent(location.href)}`;
        } else {
            history.push(`/login?return_url=${encodeURIComponent(location.href)}`);
        }
    }
});

export class App extends React.Component<
    {},
    {popupProps: PopupProps; showVersionPanel: boolean; error: Error; navItems: NavItem[]; routes: Routes; extensionsLoaded: boolean; authSettings: AuthSettings}
> {
    public static childContextTypes = {
        history: PropTypes.object,
        apis: PropTypes.object
    };

    public static getDerivedStateFromError(error: Error) {
        return {error};
    }

    private popupManager: PopupManager;
    private notificationsManager: NotificationsManager;
    private navigationManager: NavigationManager;
    private navItems: NavItem[];
    private routes: Routes;

    constructor(props: {}) {
        super(props);
        this.state = {popupProps: null, error: null, showVersionPanel: false, navItems: [], routes: null, extensionsLoaded: false, authSettings: null};
        this.popupManager = new PopupManager();
        this.notificationsManager = new NotificationsManager();
        this.navigationManager = new NavigationManager(history);
        this.navItems = navItems;
        this.routes = routes;
        services.extensions.addEventListener('systemLevel', this.onAddSystemLevelExtension.bind(this));
    }

    public async componentDidMount() {
        this.popupManager.popupProps.subscribe(popupProps => this.setState({popupProps}));
        const authSettings = await services.authService.settings();
        const {trackingID, anonymizeUsers} = authSettings.googleAnalytics || {trackingID: '', anonymizeUsers: true};
        const {loggedIn, username} = await services.users.get();
        if (trackingID) {
            const ga = await import('react-ga');
            ga.initialize(trackingID);
            const trackPageView = () => {
                if (loggedIn && username) {
                    const userId = !anonymizeUsers ? username : hashCode(username).toString();
                    ga.set({userId});
                }
                ga.pageview(location.pathname + location.search);
            };
            trackPageView();
            history.listen(trackPageView);
        }
        if (authSettings.uiCssURL) {
            const link = document.createElement('link');
            link.href = authSettings.uiCssURL;
            link.rel = 'stylesheet';
            link.type = 'text/css';
            document.head.appendChild(link);
        }

        this.setState({...this.state, navItems: this.navItems, routes: this.routes, extensionsLoaded: false, authSettings});
    }

    public render() {
        if (this.state.error != null) {
            const stack = this.state.error.stack;
            const url = 'https://github.com/argoproj/argo-cd/issues/new?labels=bug&template=bug_report.md';

            return (
                <React.Fragment>
                    <p>Something went wrong!</p>
                    <p>
                        Consider submitting an issue <a href={url}>here</a>.
                    </p>
                    <br />
                    <p>Stacktrace:</p>
                    <pre>{stack}</pre>
                </React.Fragment>
            );
        }

        return (
            <React.Fragment>
                <Helmet>
                    <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-32x32.png`} sizes='32x32' />
                    <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-16x16.png`} sizes='16x16' />
                </Helmet>
                <PageContext.Provider value={{title: 'Argo CD'}}>
                    <Provider value={{history, popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager, baseHref: base}}>
                        <DataLoader load={() => services.viewPreferences.getPreferences()}>
                            {pref => <div className={pref.theme ? 'theme-' + pref.theme : 'theme-light'}>{this.state.popupProps && <Popup {...this.state.popupProps} />}</div>}
                        </DataLoader>
                        <AuthSettingsCtx.Provider value={this.state.authSettings}>
                            <Router history={history}>
                                <Switch>
                                    <Redirect exact={true} path='/' to='/applications' />
                                    {Object.keys(this.routes).map(path => {
                                        const route = this.routes[path];
                                        return (
                                            <Route
                                                key={path}
                                                path={path}
                                                render={routeProps =>
                                                    route.noLayout ? (
                                                        <div>
                                                            <route.component {...routeProps} />
                                                        </div>
                                                    ) : (
                                                        <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                            {pref => (
                                                                <Layout onVersionClick={() => this.setState({showVersionPanel: true})} navItems={this.navItems} pref={pref}>
                                                                    <Banner>
                                                                        <route.component {...routeProps} />
                                                                    </Banner>
                                                                </Layout>
                                                            )}
                                                        </DataLoader>
                                                    )
                                                }
                                            />
                                        );
                                    })}
                                    {this.state.extensionsLoaded && <Redirect path='*' to='/' />}
                                </Switch>
                            </Router>
                        </AuthSettingsCtx.Provider>
                    </Provider>
                </PageContext.Provider>
                <Notifications notifications={this.notificationsManager.notifications} />
                <VersionPanel version={versionLoader} isShown={this.state.showVersionPanel} onClose={() => this.setState({showVersionPanel: false})} />
            </React.Fragment>
        );
    }

    public getChildContext() {
        return {history, apis: {popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager}};
    }

    private onAddSystemLevelExtension(extension: SystemLevelExtension) {
        const extendedNavItems = this.navItems;
        const extendedRoutes = this.routes;
        extendedNavItems.push({
            title: extension.title,
            path: extension.path,
            iconClassName: `fa ${extension.icon}`
        });
        const component = () => (
            <>
                <Helmet>
                    <title>{extension.title} - Argo CD</title>
                </Helmet>
                <Page title={extension.title}>
                    <extension.component />
                </Page>
            </>
        );
        extendedRoutes[extension.path] = {
            component: component as React.ComponentType<React.ComponentProps<any>>
        };
        this.setState({...this.state, navItems: extendedNavItems, routes: extendedRoutes, extensionsLoaded: true});
    }
}
