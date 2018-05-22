import { AppContext as ArgoAppContext  } from 'argo-ui';

import { NotificationManager } from './components/notification-manager';

export interface AppContext extends ArgoAppContext {
    notificationManager: NotificationManager;
}
