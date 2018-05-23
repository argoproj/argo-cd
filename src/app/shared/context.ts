import { AppContext as ArgoAppContext  } from 'argo-ui';
import * as H from 'history';

import { PopupApi } from './components';
import { NotificationManager } from './components/notification-manager';

export interface AppContext extends ArgoAppContext {
    notificationManager: NotificationManager;
    apis: {
        popup: PopupApi;
    };
    history: H.History;
}
