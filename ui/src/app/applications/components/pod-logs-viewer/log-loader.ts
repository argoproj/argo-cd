import {DataLoader} from 'argo-ui';
import * as models from '../../../shared/models';

export type LogLoader = DataLoader<models.LogEntry[], string>;
