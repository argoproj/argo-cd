import { NotificationInfo } from 'argo-ui';

export interface NotificationManager {
    showNotification(notification: NotificationInfo, autoHideMs?: number): void;
    closeNotification(notification: NotificationInfo): void;
}
