import * as path from 'path';
import * as agent from 'superagent';

import {BehaviorSubject, fromEvent, Observable, Observer} from 'rxjs';
import {debounceTime, filter} from 'rxjs/operators';

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
    DONE = 4
}

let baseHRef = '/';

const onError = new BehaviorSubject<agent.ResponseError>(null);

function toAbsURL(val: string): string {
    return path.join(baseHRef, val);
}

function apiRoot(): string {
    return toAbsURL('/api/v1');
}

function initHandlers(req: agent.Request) {
    req.on('error', err => onError.next(err));
    return req;
}

export default {
    setBaseHRef(val: string) {
        baseHRef = val;
    },
    agent,
    toAbsURL,
    onError: onError.asObservable().pipe(filter(err => err != null)),
    get(url: string) {
        return initHandlers(agent.get(`${apiRoot()}${url}`));
    },

    post(url: string) {
        return initHandlers(agent.post(`${apiRoot()}${url}`)).set('Content-Type', 'application/json');
    },

    put(url: string) {
        return initHandlers(agent.put(`${apiRoot()}${url}`)).set('Content-Type', 'application/json');
    },

    patch(url: string) {
        return initHandlers(agent.patch(`${apiRoot()}${url}`)).set('Content-Type', 'application/json');
    },

    delete(url: string) {
        return initHandlers(agent.del(`${apiRoot()}${url}`)).set('Content-Type', 'application/json');
    },

    loadEventSource(url: string): Observable<string> {
        return Observable.create((observer: Observer<any>) => {
            const fullUrl = `${apiRoot()}${url}`;

            let eventSource: EventSource = null;
            let abortController: AbortController = null;
            let interval: any = null;
            let errored = false;

            // EventSource's error event is opaque (no status, no body). On failure,
            // issue a one-shot fetch so we can surface the server's HTTP status and
            // body. This replaces the always-on prefetch, which doubled the open
            // watch streams per subscription and held an HTTP/1.1 connection slot
            // for the lifetime of every watch — see issue #27877.
            const probeAndError = (eventSourceError: any) => {
                if (errored || !abortController) {
                    return;
                }
                errored = true;
                fetch(fullUrl, {signal: abortController.signal})
                    .then(async response => {
                        if (!response.ok) {
                            const text = await response.text();
                            observer.error({status: response.status, statusText: response.statusText, body: text});
                            onError.next({status: response.status, name: response.statusText, message: text} as agent.ResponseError);
                        } else {
                            await response.body?.cancel();
                            observer.error(eventSourceError);
                            onError.next(eventSourceError);
                        }
                    })
                    .catch(fetchErr => {
                        if (fetchErr.name === 'AbortError') {
                            return;
                        }
                        observer.error(fetchErr);
                        onError.next(fetchErr);
                    });
            };

            const connect = () => {
                if (eventSource) {
                    return;
                }
                errored = false;
                abortController = new AbortController();
                eventSource = new EventSource(fullUrl);
                eventSource.onmessage = msg => observer.next(msg.data);
                eventSource.onerror = e => probeAndError(e);

                // EventSource does not provide easy way to get notification when connection closed.
                // check readyState periodically instead.
                interval = setInterval(() => {
                    if (eventSource && eventSource.readyState === ReadyState.CLOSED) {
                        probeAndError('connection got closed unexpectedly');
                    }
                }, 500);
            };

            const disconnect = () => {
                clearInterval(interval);
                interval = null;
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                if (abortController) {
                    abortController.abort();
                    abortController = null;
                }
            };

            // Browsers allow only a handful of concurrent HTTP/1.1 connections per origin (6 in
            // Chrome) and every stream holds one of them open for as long as it lives, while the
            // API server only speaks HTTP/1.1 to browsers. A couple of tabs left on an app page or
            // on a log view are therefore enough to use up the whole pool, and every other request
            // — including the can-i checks the Logs and Exec entries wait for — queues forever.
            // A stream nobody is looking at is worth nothing, so hand its connection back while the
            // tab is hidden and reconnect when the user returns. See issue #26532.
            if (!document.hidden) {
                connect();
            }
            const visibilityChange = fromEvent(document, 'visibilitychange')
                // wait until user stops clicking back and forth to avoid reconnecting too often
                .pipe(debounceTime(500))
                .subscribe(() => (document.hidden ? disconnect() : connect()));

            return () => {
                visibilityChange.unsubscribe();
                disconnect();
            };
        });
    }
};
