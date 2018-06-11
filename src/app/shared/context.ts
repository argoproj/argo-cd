import { AppContext as ArgoAppContext, NotificationsApi, PopupApi } from 'argo-ui';
import { NavigationApi } from './navigation';

export type AppContext = ArgoAppContext & { apis: { popup: PopupApi; notifications: NotificationsApi; navigation: NavigationApi }; };
