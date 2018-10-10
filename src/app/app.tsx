import { Layout, NavigationManager, NotificationInfo, Notifications, NotificationsManager, Popup, PopupManager, PopupProps } from 'argo-ui';
import * as cookie from 'cookie';
import createHistory from 'history/createBrowserHistory';
import * as jwtDecode from 'jwt-decode';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Redirect, Route, RouteComponentProps, Router, Switch } from 'react-router';

import { services } from './shared/services';
import requests from './shared/services/requests';

services.viewPreferences.init();
export const history = createHistory();

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
    title: 'Apps',
    path: '/applications',
    iconClassName: 'argo-icon-application',
}, {
    title: 'Settings',
    path: '/settings',
    iconClassName: 'argo-icon-settings',
}, {
    title: 'Help',
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
                return (authSettings.dexConfig && authSettings.dexConfig.connectors || []).length > 0;
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
            if (await isExpiredSSO()) {
                window.location.pathname = `/auth/login?return_url=${encodeURIComponent(location.href)}`;
            } else {
                history.push(`/login?return_url=${encodeURIComponent(location.href)}`);
            }
        }
    }
});

export class App extends React.Component<{}, { notifications: NotificationInfo[], popupProps: PopupProps }> {
    public static childContextTypes = {
        history: PropTypes.object,
        apis: PropTypes.object,
    };

    private popupManager: PopupManager;
    private notificationsManager: NotificationsManager;
    private navigationManager: NavigationManager;

    constructor(props: {}) {
        super(props);
        this.state = { notifications: [], popupProps: null };
        this.popupManager = new PopupManager();
        this.notificationsManager = new NotificationsManager();
        this.navigationManager = new NavigationManager(history);
    }

    public componentDidMount() {
        this.popupManager.popupProps.subscribe((popupProps) => this.setState({ popupProps }));
        this.notificationsManager.notifications.subscribe((notifications) => this.setState({ notifications }));
    }

    public render() {
        return (
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
                                        <Notifications leftOffset={60}
                                            closeNotification={(item) => this.notificationsManager.close(item)} notifications={this.state.notifications}/>
                                        <route.component {...routeProps}/>
                                    </div>
                                ) : (
                                    <Layout navItems={navItems}>
                                        <Notifications leftOffset={60}
                                            closeNotification={(item) => this.notificationsManager.close(item)} notifications={this.state.notifications}/>
                                        <route.component {...routeProps}/>
                                    </Layout>
                                )
                            )}/>;
                        })}
                        <Redirect path='*' to='/'/>
                    </Switch>
                </Router>
            </Provider>
        );
    }

    public getChildContext() {
        return { history, apis: { popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager } };
    }
}
