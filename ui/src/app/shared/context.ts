import {AppContext as ArgoAppContext, NavigationApi, NotificationsApi, PopupApi} from 'argo-ui';
import {Location, LocationState, Path} from 'history';
import * as React from 'react';
import * as models from './models';

export type AppContext = ArgoAppContext & {apis: {popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi; baseHref: string}};

export interface ContextApis {
    popup: PopupApi;
    notifications: NotificationsApi;
    navigation: NavigationApi;
    baseHref: string;
}

// This is a subset of the History interface imported from 'history', which is
// currently the only part of the History interface which the Login component
// uses. This is used to simplify testing.
export interface MinimalHistory<HistoryLocationState = LocationState> {
    location: Location<HistoryLocationState>;
    push(path: Path, state?: HistoryLocationState): void;
}

export const Context = React.createContext<ContextApis & {history: MinimalHistory}>(null);
export let {Provider, Consumer} = Context;

export const AuthSettingsCtx = React.createContext<models.AuthSettings>(null);
