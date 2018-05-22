import { NotificationInfo } from 'argo-ui';

export interface NotificationManager {
    showNotification(notification: NotificationInfo): void;
    closeNotification(notification: NotificationInfo): void;
}
