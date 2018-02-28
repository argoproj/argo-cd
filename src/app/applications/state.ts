import { Subscription } from 'rxjs';
import * as models from '../shared/models';

export interface State {
    applications?: models.Application[];
    application?: models.Application;
    changesSubscription?: Subscription;
}
