import * as _superagent from 'superagent';
const superagentPromise = require('superagent-promise');
import { Observable, Observer } from 'rxjs';

type Callback = (data: any) => void;

declare class EventSource {
    public onopen: Callback;
    public onmessage: Callback;
    public onerror: Callback;
    public readyState: number;
    constructor(url: string);
    public close(): void;
}

enum ReadyState {
  CONNECTING = 0,
  OPEN = 1,
  CLOSED = 2,
  DONE = 4,
}

const superagent: _superagent.SuperAgentStatic = superagentPromise(_superagent, global.Promise);

const API_ROOT = '/api/v1';

export default {
    get(url: string) {
        return superagent.get(`${API_ROOT}${url}`);
    },

    post(url: string) {
        return superagent.post(`${API_ROOT}${url}`);
    },

    delete(url: string) {
        return superagent.del(`${API_ROOT}${url}`);
    },

    loadEventSource(url: string, allowAutoRetry = false): Observable<string> {
        return Observable.create((observer: Observer<any>) => {
            const eventSource = new EventSource(`${API_ROOT}${url}`);
            let opened = false;
            eventSource.onopen = (msg) => {
                if (!opened) {
                    opened = true;
                } else if (!allowAutoRetry) {
                    eventSource.close();
                    observer.complete();
                }
            };
            eventSource.onmessage = (msg) => observer.next(msg.data);
            eventSource.onerror = (e) => () => {
                if (e.eventPhase === ReadyState.CLOSED || eventSource.readyState === ReadyState.CONNECTING) {
                    observer.complete();
                } else {
                    observer.error(e);
                }
            };
            return () => {
                eventSource.close();
            };
        });
    },
};
