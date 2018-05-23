import { AppContext as ArgoAppContext  } from 'argo-ui';
import * as H from 'history';

import { NotificationManager } from './components/notification-manager';

export interface AppContext extends ArgoAppContext {
    notificationManager: NotificationManager;
    history: H.History;
}
