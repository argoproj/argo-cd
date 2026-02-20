import {DataLoader, NavigationManager, Notifications, NotificationsManager, PageContext, Popup, PopupManager, PopupProps} from 'argo-ui';
import {createBrowserHistory} from 'history';
import React, {useState, useRef, useCallback, useEffect, Component, ReactNode, FC, ComponentType, ComponentProps} from 'react';
import {Helmet} from 'react-helmet';
import {Redirect, Route, RouteComponentProps, Router, Switch} from 'react-router';
import {Subscription} from 'rxjs';
import applications from './applications';
import help from './help';
import login from './login';
import settings from './settings';
import {Layout, ThemeWrapper} from './shared/components/layout/layout';
import {Page} from './shared/components/page/page';
import {VersionPanel} from './shared/components/version-info/version-info-panel';
import {AuthSettingsCtx, Provider} from './shared/context';
import {services} from './shared/services';
import requests from './shared/services/requests';
import {hashCode} from './shared/utils';
import {Banner} from './ui-banner/ui-banner';
import userInfo from './user-info';
import {AuthSettings} from './shared/models';
import {SystemLevelExtension} from './shared/services/extensions-service';

services.viewPreferences.init();
const bases = document.getElementsByTagName('base');
const base = bases.length > 0 ? bases[0].getAttribute('href') || '/' : '/';
export const history = createBrowserHistory({basename: base});
requests.setBaseHRef(base);

type Routes = {[path: string]: {component: ComponentType<RouteComponentProps<any>>; noLayout?: boolean}};

const routes: Routes = {
    '/login': {component: login.component as any, noLayout: true},
    '/applications': {component: applications.component},
    // TODO: Uncomment when ApplicationSet details page is fully implemented
    // '/applicationsets': {component: applications.component},
    '/settings': {component: settings.component},
    '/user-info': {component: userInfo.component},
    '/help': {component: help.component}
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

interface ErrorBoundaryProps {
    children: ReactNode;
}

interface ErrorBoundaryState {
    hasError: boolean;
    error: Error | null;
}

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
    constructor(props: ErrorBoundaryProps) {
        super(props);
        this.state = {hasError: false, error: null};
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return {hasError: true, error};
    }

    render() {
        if (this.state.hasError) {
            const stack = this.state.error?.stack;
            const url = 'https://github.com/argoproj/argo-cd/issues/new?labels=bug&template=bug_report.md';

            return (
                <>
                    <p>Something went wrong!</p>
                    <p>
                        Consider submitting an issue <a href={url}>here</a>.
                    </p>
                    <br />
                    <p>Stacktrace:</p>
                    <pre>{stack}</pre>
                </>
            );
        }

        return this.props.children;
    }
}

const AppContent: FC = () => {
    const [popupProps, setPopupProps] = useState<PopupProps>(null);
    const [showVersionPanel, setShowVersionPanel] = useState(false);
    const [navItemsState, setNavItemsState] = useState<NavItem[]>(navItems);
    const [routesState, setRoutesState] = useState<Routes>(routes);
    const [authSettings, setAuthSettings] = useState<AuthSettings>(null);

    const popupManager = useRef(new PopupManager());
    const notificationsManager = useRef(new NotificationsManager());
    const navigationManager = useRef(new NavigationManager(history));

    const subscribeUnauthorized = useCallback(async () => {
        return requests.onError.subscribe(async err => {
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
    }, []);

    const onAddSystemLevelExtension = useCallback((extension: SystemLevelExtension) => {
        setNavItemsState(prev => {
            const extendedNavItems = [...prev];
            extendedNavItems.push({
                title: extension.title,
                path: extension.path,
                iconClassName: `fa ${extension.icon}`
            });
            return extendedNavItems;
        });

        setRoutesState(prev => {
            const extendedRoutes = {...prev};
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
                component: component as ComponentType<ComponentProps<any>>
            };
            return extendedRoutes;
        });
    }, []);

    useEffect(() => {
        services.extensions.addEventListener('systemLevel', onAddSystemLevelExtension);

        return () => {
            // Note: services.extensions might not have removeEventListener method
            // If it does, uncomment the following line:
            // services.extensions.removeEventListener('systemLevel', onAddSystemLevelExtension);
        };
    }, [onAddSystemLevelExtension]);

    useEffect(() => {
        const popupPropsSubscription = popupManager.current.popupProps.subscribe(setPopupProps);
        let unauthorizedSubscription: Subscription;

        subscribeUnauthorized().then(subscription => {
            unauthorizedSubscription = subscription;
        });

        const initializeApp = async () => {
            const authSettingsData = await services.authService.settings();
            const {trackingID, anonymizeUsers} = authSettingsData.googleAnalytics || {trackingID: '', anonymizeUsers: true};
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

            if (authSettingsData.uiCssURL) {
                const link = document.createElement('link');
                link.href = authSettingsData.uiCssURL;
                link.rel = 'stylesheet';
                link.type = 'text/css';
                document.head.appendChild(link);
            }

            setAuthSettings(authSettingsData);
        };

        initializeApp();

        return () => {
            if (popupPropsSubscription) {
                popupPropsSubscription.unsubscribe();
            }
            if (unauthorizedSubscription) {
                unauthorizedSubscription.unsubscribe();
            }
        };
    }, [subscribeUnauthorized]);

    return (
        <>
            <Helmet>
                <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-32x32.png`} sizes='32x32' />
                <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-16x16.png`} sizes='16x16' />
            </Helmet>
            <PageContext.Provider value={{title: 'Argo CD'}}>
                <Provider value={{history, popup: popupManager.current, notifications: notificationsManager.current, navigation: navigationManager.current, baseHref: base}}>
                    <DataLoader load={() => services.viewPreferences.getPreferences()}>
                        {pref => <ThemeWrapper theme={pref.theme}>{popupProps && <Popup {...popupProps} />}</ThemeWrapper>}
                    </DataLoader>
                    <AuthSettingsCtx.Provider value={authSettings}>
                        <Router history={history}>
                            <Switch>
                                <Redirect exact={true} path='/' to='/applications' />
                                {Object.keys(routesState).map(path => {
                                    const route = routesState[path];
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
                                                            <Layout onVersionClick={() => setShowVersionPanel(true)} navItems={navItemsState} pref={pref}>
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
                            </Switch>
                        </Router>
                    </AuthSettingsCtx.Provider>
                </Provider>
            </PageContext.Provider>
            <Notifications notifications={notificationsManager.current.notifications} />
            <VersionPanel version={versionLoader} isShown={showVersionPanel} onClose={() => setShowVersionPanel(false)} />
        </>
    );
};

export const App: FC = () => {
    return (
        <ErrorBoundary>
            <AppContent />
        </ErrorBoundary>
    );
};
