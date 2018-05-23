import { asyncMiddleware, getReducer, Layout, NotificationInfo, Notifications, RouteImplementation } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Provider } from 'react-redux';

import createHistory from 'history/createBrowserHistory';
import { Redirect, Route, Switch } from 'react-router';
import { ConnectedRouter, push, routerMiddleware} from 'react-router-redux';
import { applyMiddleware, createStore, Store } from 'redux';

import { NotificationManager } from './shared/components';
import requests from './shared/services/requests';

export const history = createHistory();
const reduxRouterMiddleware = routerMiddleware(history);

const noopReducer = (state: any, action: any) => {
    // do nothing
};

import applications from './applications';
import help from './help';
import login from './login';
const routes: {[path: string]: RouteImplementation & { noLayout?: boolean } } = {
    '/applications': { component: applications.component, reducer: noopReducer },
    '/login': { component: login.component as any, reducer: noopReducer, noLayout: true },
    '/help': { component: help.component, reducer: noopReducer },
};

const navItems = [{
    title: 'Apps',
    path: '/applications',
    iconClassName: 'argo-icon-application',
}, {
    title: 'Help',
    path: '/help',
    iconClassName: 'argo-icon-docs',
}];

const routesReducer = getReducer(routes);
export const store = createStore(routesReducer, applyMiddleware(asyncMiddleware, reduxRouterMiddleware));

requests.onError.subscribe((err) => {
    if (err.status === 401) {
        store.dispatch(push(`/login?return_url=${encodeURIComponent(location.href)}`));
    }
});

export class App extends React.Component<{store: Store<any>}, { notifications: NotificationInfo[] }> implements NotificationManager {
    public static childContextTypes = {
        notificationManager: PropTypes.object,
    };

    constructor(props: {store: Store<any>}) {
        super(props);
        this.state = { notifications: [] };
    }

    public showNotification(notification: NotificationInfo, autoHideMs = 5000) {
        this.setState({ notifications: [...(this.state.notifications || []), notification] });
        if (autoHideMs > -1) {
            setTimeout(() => this.closeNotification(notification), autoHideMs);
        }
    }

    public closeNotification(notification: NotificationInfo): void {
        const notifications = (this.state.notifications || []).slice();
        const index = this.state.notifications.indexOf(notification);
        if (index > -1) {
            notifications.splice(index, 1);
            this.setState({notifications});
        }
    }

    public render() {
        return (
            <Provider store={this.props.store}>
                <ConnectedRouter history={history} store={this.props.store}>
                    <Switch>
                        <Redirect exact={true} path='/' to='/applications'/>
                        {Object.keys(routes).map((path) => {
                            const route = routes[path];
                            return <Route key={path} path={path} render={(routeProps) => (
                                route.noLayout ? (
                                    <div>
                                        <Notifications leftOffset={60} closeNotification={(item) => this.closeNotification(item)} notifications={this.state.notifications}/>
                                        <route.component {...routeProps}/>
                                    </div>
                                ) : (
                                    <Layout navItems={navItems}>
                                        <Notifications leftOffset={60} closeNotification={(item) => this.closeNotification(item)} notifications={this.state.notifications}/>
                                        <route.component {...routeProps}/>
                                    </Layout>
                                )
                            )}/>;
                        })}
                    </Switch>
                </ConnectedRouter>
            </Provider>
        );
    }

    public getChildContext() {
        return { notificationManager: this };
    }
}
