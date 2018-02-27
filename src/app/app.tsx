import { asyncMiddleware, getRoutesReducer, Layout, RouteImplementation } from 'argo-ui';
import * as React from 'react';
import { Provider } from 'react-redux';

import createHistory from 'history/createBrowserHistory';
import { Redirect, Route, Switch } from 'react-router';
import { ConnectedRouter, routerMiddleware} from 'react-router-redux';
import { applyMiddleware, createStore, Store } from 'redux';

export const history = createHistory();
const reduxRouterMiddleware = routerMiddleware(history);

import applications from './applications';
const routes: {[path: string]: RouteImplementation } = {
    '/applications': { component: applications.component, reducer: applications.reducer },
};

const navItems = [{
    title: 'Apps',
    path: '/applications',
    iconClassName: 'argo-icon-application',
}];

const reducer = getRoutesReducer(routes);
export const store = createStore(reducer, applyMiddleware(asyncMiddleware, reduxRouterMiddleware));

export const App = (props: {store: Store<any>}) => (
    <Provider store={props.store}>
        <ConnectedRouter history={history} store={props.store}>
            <Switch>
                <Redirect exact={true} path='/' to='/applications'/>
                <Layout navItems={navItems}>
                    {Object.keys(routes).map((path) => {
                        const route = routes[path];
                        return <Route key={path} path={path} component={route.component}/>;
                    })}
                </Layout>
            </Switch>
        </ConnectedRouter>
    </Provider>
);
