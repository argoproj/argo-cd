import * as H from 'history';
import { match, RouteComponentProps } from 'react-router';
import { routerReducer, RouterState } from 'react-router-redux';
import { Reducer } from 'redux';

export interface AppState<S> {
    router?: RouterState;
    page?: S;
}

export interface AppContext {
    router: {
        history: H.History;
        route: {
            location: H.Location;
            match: match<any>;
        };
    };
}

export interface RouteImplementation {
    reducer: Reducer<any>;
    component: React.ComponentType<RouteComponentProps<any>>;
}

export function isActiveRoute(locationPath: string, path: string) {
    return locationPath === path || locationPath.startsWith(`${path}/`);
}

export function getRoutesReducer(routes: {[path: string]: RouteImplementation }) {
    return (state: AppState<any> = {}, action: any) => {
        const nextState = {...state};
        nextState.router = routerReducer(nextState.router, action);
        const locationPath = nextState.router
            && nextState.router.location
            && nextState.router.location.pathname || '';
        const pageReducerPath = Object.keys(routes).find((path) => isActiveRoute(locationPath, path));
        const pageReducer = pageReducerPath && routes[pageReducerPath];
        if (pageReducer) {
            nextState.page = pageReducer.reducer(nextState.page, action);
        }
        return nextState;
    };
}

export const asyncMiddleware = ({ dispatch, getState }: any) => (next: any) => (action: any) => {
    if (typeof action === 'function') {
        return action(dispatch, getState);
    }

    return next(action);
};
