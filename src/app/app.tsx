import { Layout, NavigationManager, Notifications, NotificationsManager, PageContext, Popup, PopupManager, PopupProps } from 'argo-ui';
import * as cookie from 'cookie';
import { createBrowserHistory } from 'history';
import * as jwtDecode from 'jwt-decode';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Helmet } from 'react-helmet';
import { Redirect, Route, RouteComponentProps, Router, Switch } from 'react-router';

import { services } from './shared/services';
import requests from './shared/services/requests';

services.viewPreferences.init();
const bases = document.getElementsByTagName('base');
const base = bases.length > 0 ? bases[0].getAttribute('href') || '/' : '/';
export const history = createBrowserHistory({ basename: base });
requests.setApiRoot(`${base}api/v1`);

import applications from './applications';
import help from './help';
import login from './login';
import settings from './settings';
import { Provider } from './shared/context';

const routes: {[path: string]: { component: React.ComponentType<RouteComponentProps<any>>, noLayout?: boolean } } = {
    '/login': { component: login.component as any, noLayout: true },
    '/applications': { component: applications.component },
    '/settings': { component: settings.component },
    '/help': { component: help.component },
};

const navItems = [{
    title: 'Manage your applications, and diagnose health problems.',
    path: '/applications',
    iconClassName: 'argo-icon-application',
}, {
    title: 'Manage your repositories, projects, settings',
    path: '/settings',
    iconClassName: 'argo-icon-settings',
}, {
    title: 'Read the documentation, and get help and assistance.',
    path: '/help',
    iconClassName: 'argo-icon-docs',
}];

async function isExpiredSSO() {
    try {
        const token = cookie.parse(document.cookie)['argocd.token'];
        if (token) {
            const jwtToken = jwtDecode(token) as any;
            if (jwtToken.iss && jwtToken.iss !== 'argocd') {
                const authSettings = await services.authService.settings();
                return (authSettings.dexConfig && authSettings.dexConfig.connectors || []).length > 0 || authSettings.oidcConfig;
            }
        }
    } catch {
        return false;
    }
    return false;
}

requests.onError.subscribe(async (err) => {
    if (err.status === 401) {
        if (!history.location.pathname.startsWith('/login')) {
            // Query for basehref and remove trailing /.
            // If basehref is the default `/` it will become an empty string.
            const basehref = document.querySelector('head > base').getAttribute('href').replace(/\/$/, '');
            if (await isExpiredSSO()) {
                window.location.href = `${basehref}/auth/login?return_url=${encodeURIComponent(location.href)}`;
            } else {
                history.push(`${basehref}/login?return_url=${encodeURIComponent(location.href)}`);
            }
        }
    }
});

export class App extends React.Component<{}, { popupProps: PopupProps }> {
    public static childContextTypes = {
        history: PropTypes.object,
        apis: PropTypes.object,
    };

    private popupManager: PopupManager;
    private notificationsManager: NotificationsManager;
    private navigationManager: NavigationManager;

    constructor(props: {}) {
        super(props);
        this.state = { popupProps: null };
        this.popupManager = new PopupManager();
        this.notificationsManager = new NotificationsManager();
        this.navigationManager = new NavigationManager(history);
    }

    public componentDidMount() {
        this.popupManager.popupProps.subscribe((popupProps) => this.setState({ popupProps }));
    }

    public render() {
        return (
            <React.Fragment>
                <Helmet>
                    <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-32x32.png`} sizes='32x32'/>
                    <link rel='icon' type='image/png' href={`${base}assets/favicon/favicon-16x16.png`} sizes='16x16'/>
                </Helmet>
                <PageContext.Provider value={{title: 'Argo CD'}}>
                <Provider value={{history, popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager}}>
                    {this.state.popupProps && <Popup {...this.state.popupProps}/>}
                    <Router history={history}>
                        <Switch>
                            <Redirect exact={true} path='/' to='/applications'/>
                            {Object.keys(routes).map((path) => {
                                const route = routes[path];
                                return <Route key={path} path={path} render={(routeProps) => (
                                    route.noLayout ? (
                                        <div>
                                            <route.component {...routeProps}/>
                                        </div>
                                    ) : (
                                        <Layout navItems={navItems}>
                                            <route.component {...routeProps}/>
                                        </Layout>
                                    )
                                )}/>;
                            })}
                            <Redirect path='*' to='/'/>
                        </Switch>
                    </Router>
                </Provider>
                </PageContext.Provider>
                <Notifications notifications={this.notificationsManager.notifications}/>
            </React.Fragment>
        );
    }

    public getChildContext() {
        return { history, apis: { popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager } };
    }
}
