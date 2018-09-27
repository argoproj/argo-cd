import { AppContext as ArgoAppContext, NavigationApi, NotificationsApi, PopupApi } from 'argo-ui';
import { History } from 'history';
import * as React from 'react';

export type AppContext = ArgoAppContext & { apis: { popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi }; };

export const { Provider, Consumer } = React.createContext<{ popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi; history: History }>(null);
