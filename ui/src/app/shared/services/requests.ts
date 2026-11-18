import * as path from 'path';
import * as agent from 'superagent';

import {BehaviorSubject, Observable, Observer} from 'rxjs';
import {filter} from 'rxjs/operators';

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

// How long loadEventSource waits for the HTTP status of a stream that the server refused.
export const STREAM_STATUS_TIMEOUT = 3000;

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

            const abortController = new AbortController();
            let errored = false;
            let statusTimeout: ReturnType<typeof setTimeout>;

            const fail = (err: any) => {
                observer.error(err);
                onError.next(err);
            };

            // EventSource's error event is opaque (no status, no body). When the server refused the stream,
            // issue a one-shot fetch so we can surface its HTTP status and body. This replaces the always-on
            // prefetch, which doubled the open watch streams per subscription and held an HTTP/1.1 connection
            // slot for the lifetime of every watch — see issue #27877.
            const failWithStatus = (eventSourceError: any) => {
                // The fetch is the same stream again, so it can be accepted and then stay silent: a watch may
                // not answer until it has something to send. Give up on the status then, not on the stream.
                // Nothing is listening until the subscriber gets the error and reconnects.
                statusTimeout = setTimeout(() => {
                    abortController.abort();
                    fail(eventSourceError);
                }, STREAM_STATUS_TIMEOUT);
                fetch(fullUrl, {signal: abortController.signal})
                    .then(async response => {
                        if (abortController.signal.aborted) {
                            return;
                        }
                        clearTimeout(statusTimeout);
                        if (!response.ok) {
                            const text = await response.text();
                            observer.error({status: response.status, statusText: response.statusText, body: text});
                            onError.next({status: response.status, name: response.statusText, message: text} as agent.ResponseError);
                        } else {
                            await response.body?.cancel();
                            fail(eventSourceError);
                        }
                    })
                    .catch(fetchErr => {
                        // Aborted by the timeout, which reported the error, or by the subscriber leaving.
                        if (abortController.signal.aborted) {
                            return;
                        }
                        clearTimeout(statusTimeout);
                        fail(fetchErr);
                    });
            };

            const onStreamError = (eventSourceError: any) => {
                if (errored) {
                    return;
                }
                errored = true;
                // The browser closes a stream for good when the server answers it with an error status, which
                // is the only case with a status to learn. After a dropped connection it is reconnecting.
                const refused = eventSource.readyState === ReadyState.CLOSED;
                // Reconnecting is up to the subscriber, which may have to reload what it missed. Don't let the
                // browser reconnect as well: the subscriber's stream would come on top of it.
                eventSource.close();
                if (refused) {
                    failWithStatus(eventSourceError);
                } else {
                    fail(eventSourceError);
                }
            };

            let eventSource = new EventSource(fullUrl);
            eventSource.onmessage = msg => observer.next(msg.data);
            eventSource.onerror = e => onStreamError(e);

            // EventSource does not provide easy way to get notification when connection closed.
            // check readyState periodically instead.
            const interval = setInterval(() => {
                if (eventSource && eventSource.readyState === ReadyState.CLOSED) {
                    onStreamError('connection got closed unexpectedly');
                }
            }, 500);
            return () => {
                clearInterval(interval);
                clearTimeout(statusTimeout);
                eventSource.close();
                abortController.abort();
                eventSource = null;
            };
        });
    }
};
