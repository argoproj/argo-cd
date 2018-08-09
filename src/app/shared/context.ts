import { AppContext as ArgoAppContext, NotificationsApi, PopupApi } from 'argo-ui';
import { History } from 'history';
import * as React from 'react';

import { NavigationApi } from './navigation';

export type AppContext = ArgoAppContext & { apis: { popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi }; };

export const { Provider, Consumer } = React.createContext<{ popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi; history: History }>(null);
