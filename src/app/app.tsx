import { asyncMiddleware, getReducer, Layout, NotificationsContainer, RouteImplementation } from 'argo-ui';
import * as React from 'react';
import { Provider } from 'react-redux';

import createHistory from 'history/createBrowserHistory';
import { Redirect, Route, Switch } from 'react-router';
import { ConnectedRouter, push, routerMiddleware} from 'react-router-redux';
import { applyMiddleware, createStore, Store } from 'redux';
import { reducer as formReducer } from 'redux-form';

import requests from './shared/services/requests';

export const history = createHistory();
const reduxRouterMiddleware = routerMiddleware(history);

import applications from './applications';
import login from './login';
const routes: {[path: string]: RouteImplementation & { noLayout?: boolean } } = {
    '/applications': { component: applications.component, reducer: applications.reducer },
    '/login': { component: login.component as any, reducer: login.reducer, noLayout: true },
};

const navItems = [{
    title: 'Apps',
    path: '/applications',
    iconClassName: 'argo-icon-application',
}];

const routesReducer = getReducer(routes);

const reducer = (state: any, action: any) => {
    const next = routesReducer(state, action) as any;
    next.form = formReducer(next.form, action);
    return next;
};

export const store = createStore(reducer, applyMiddleware(asyncMiddleware, reduxRouterMiddleware));

requests.onError.subscribe((err) => {
    if (err.status === 401) {
        store.dispatch(push(`/login?return_url=${encodeURIComponent(location.href)}`));
    }
});

export const App = (props: {store: Store<any>}) => (
    <Provider store={props.store}>
        <ConnectedRouter history={history} store={props.store}>
            <Switch>
                <Redirect exact={true} path='/' to='/applications'/>
                {Object.keys(routes).map((path) => {
                    const route = routes[path];
                    return <Route key={path} path={path} render={(routeProps) => (
                        route.noLayout ? (
                            <div>
                                <NotificationsContainer />
                                <route.component {...routeProps}/>
                            </div>
                        ) : (
                            <Layout navItems={navItems}>
                                <NotificationsContainer />
                                <route.component {...routeProps}/>
                            </Layout>
                        )
                    )}/>;
                })}
            </Switch>
        </ConnectedRouter>
    </Provider>
);
