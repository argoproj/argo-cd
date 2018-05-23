import { NotificationInfo } from 'argo-ui';
import { BehaviorSubject, Observable } from 'rxjs';

export interface NotificationsApi {
    show(notification: NotificationInfo, autoHideMs?: number): void;
    close(notification: NotificationInfo): void;
}

export class NotificationsManager {
    private notificationsSubject: BehaviorSubject<NotificationInfo[]> = new BehaviorSubject([]);

    public get notifications(): Observable<NotificationInfo[]> {
        return this.notificationsSubject.asObservable();
    }

    public show(notification: NotificationInfo, autoHideMs = 5000) {
        this.notificationsSubject.next([...(this.notificationsSubject.getValue() || []), notification]);
        if (autoHideMs > -1) {
            setTimeout(() => this.close(notification), autoHideMs);
        }
    }

    public close(notification: NotificationInfo): void {
        const notifications = (this.notificationsSubject.getValue() || []).slice();
        const index = this.notificationsSubject.getValue().indexOf(notification);
        if (index > -1) {
            notifications.splice(index, 1);
            this.notificationsSubject.next(notifications);
        }
    }
}
