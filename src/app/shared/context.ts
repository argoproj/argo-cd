import { AppContext as ArgoAppContext  } from 'argo-ui';
import * as H from 'history';

import { PopupApi } from './components';
import { NotificationsApi } from './components/notification-manager';

export interface AppContext extends ArgoAppContext {
    apis: {
        popup: PopupApi;
        notifications: NotificationsApi;
    };
    history: H.History;
}
