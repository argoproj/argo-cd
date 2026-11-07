import {VersionMessage} from '../models';
import requests from './requests';

export class VersionService {
    private versionPromise: Promise<VersionMessage> | null = null;
    private cachedVersion: VersionMessage | null = null;

    public getCachedVersion(): VersionMessage | null {
        return this.cachedVersion;
    }

    public version(): Promise<VersionMessage> {
        if (!this.versionPromise) {
            this.versionPromise = requests.agent
                .get(requests.toAbsURL('/api/version'))
                .then(res => {
                    const version = res.body as VersionMessage;
                    this.cachedVersion = version;
                    return version;
                })
                .catch(err => {
                    this.versionPromise = null;
                    this.cachedVersion = null;
                    throw err;
                });
        }
        return this.versionPromise;
    }
}
