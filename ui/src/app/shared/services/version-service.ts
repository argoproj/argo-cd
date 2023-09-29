import {VersionMessage} from '../models';
import requests from './requests';

export class VersionService {
    public version(): Promise<VersionMessage> {
        return requests.agent.get(requests.toAbsURL('/api/version')).then(res => res.body as VersionMessage);
    }
}
