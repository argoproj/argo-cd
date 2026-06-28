import {NotificationChunk} from '../models';
import requests from './requests';

export class NotificationService {
    public listServices(): Promise<NotificationChunk[]> {
        return requests.get('/notifications/services').then(res => res.body.items || []);
    }

    public listTriggers(): Promise<NotificationChunk[]> {
        return requests.get('/notifications/triggers').then(res => res.body.items || []);
    }
}
