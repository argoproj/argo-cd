import * as path from 'path';

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

export interface ApiResponse<T = any> {
    body: T;
    status: number;
    statusText: string;
    headers: Headers;
    text: string;
}

export class ResponseError extends Error {
    public readonly status: number;
    public readonly response?: ApiResponse<any>;

    constructor(message: string, status: number, response?: ApiResponse<any>) {
        super(message);
        Object.setPrototypeOf(this, ResponseError.prototype);
        this.name = 'ResponseError';
        this.status = status;
        this.response = response;
    }
}

let baseHRef = '/';

const onError = new BehaviorSubject<ResponseError>(null);

function toAbsURL(val: string): string {
    return path.join(baseHRef, val);
}

function apiRoot(): string {
    return toAbsURL('/api/v1');
}

function appendQuery(params: URLSearchParams, value: string | Record<string, any>) {
    if (typeof value === 'string') {
        new URLSearchParams(value).forEach((v, k) => params.append(k, v));
        return;
    }
    for (const key of Object.keys(value)) {
        const v = (value as any)[key];
        if (v === undefined || v === null) {
            continue;
        }
        if (Array.isArray(v)) {
            v.forEach(item => params.append(key, String(item)));
        } else {
            params.append(key, String(v));
        }
    }
}

function parseBody(text: string, contentType: string): any {
    if (!text) {
        return null;
    }
    if (contentType.includes('application/json')) {
        try {
            return JSON.parse(text);
        } catch {
            return text;
        }
    }
    return text;
}

function buildApiResponse(response: Response, text: string): ApiResponse {
    const contentType = response.headers?.get('content-type') || '';
    return {
        body: parseBody(text, contentType),
        status: response.status,
        statusText: response.statusText,
        headers: response.headers,
        text
    };
}

function errorMessageFor(apiResponse: ApiResponse): string {
    const {body, text, statusText} = apiResponse;
    if (body && typeof body === 'object') {
        if (typeof (body as any).message === 'string') {
            return (body as any).message;
        }
        if (typeof (body as any).error === 'string') {
            return (body as any).error;
        }
    } else if (typeof body === 'string' && body) {
        return body;
    }
    return text || statusText;
}

class Request implements Promise<ApiResponse> {
    public readonly [Symbol.toStringTag] = 'Promise';

    private params = new URLSearchParams();
    private headersMap: Record<string, string> = {};
    private bodyValue: BodyInit | undefined;
    private promise: Promise<ApiResponse> | null = null;

    constructor(
        private method: string,
        private url: string
    ) {}

    set(name: string, value: string): this {
        this.headersMap[name] = value;
        return this;
    }

    query(value: string | Record<string, any>): this {
        appendQuery(this.params, value);
        return this;
    }

    send(body?: any): this {
        if (body === undefined || body === null || body === '') {
            return this;
        }
        if (body instanceof FormData) {
            // Let fetch set multipart/form-data with the right boundary.
            delete this.headersMap['Content-Type'];
            this.bodyValue = body;
            return this;
        }
        if (typeof body === 'string' || body instanceof Blob || body instanceof ArrayBuffer) {
            this.bodyValue = body as BodyInit;
            return this;
        }
        this.bodyValue = JSON.stringify(body);
        if (!this.headersMap['Content-Type']) {
            this.headersMap['Content-Type'] = 'application/json';
        }
        return this;
    }

    then<TResult1 = ApiResponse, TResult2 = never>(
        onfulfilled?: ((value: ApiResponse) => TResult1 | PromiseLike<TResult1>) | undefined | null,
        onrejected?: ((reason: any) => TResult2 | PromiseLike<TResult2>) | undefined | null
    ): Promise<TResult1 | TResult2> {
        return this.exec().then(onfulfilled, onrejected);
    }

    catch<TResult = never>(onrejected?: ((reason: any) => TResult | PromiseLike<TResult>) | undefined | null): Promise<ApiResponse | TResult> {
        return this.exec().catch(onrejected);
    }

    finally(onfinally?: (() => void) | undefined | null): Promise<ApiResponse> {
        return this.exec().finally(onfinally);
    }

    private exec(): Promise<ApiResponse> {
        if (this.promise) {
            return this.promise;
        }
        const qs = this.params.toString();
        const fullUrl = qs ? `${this.url}?${qs}` : this.url;
        this.promise = fetch(fullUrl, {
            method: this.method,
            headers: this.headersMap,
            body: this.bodyValue,
            credentials: 'same-origin'
        })
            .then(async response => {
                const text = await response.text();
                const apiResponse = buildApiResponse(response, text);
                if (!response.ok) {
                    const err = new ResponseError(errorMessageFor(apiResponse), response.status, apiResponse);
                    onError.next(err);
                    throw err;
                }
                return apiResponse;
            })
            .catch(err => {
                if (err instanceof ResponseError) {
                    throw err;
                }
                const wrapped = new ResponseError(err?.message || String(err), 0);
                wrapped.name = err?.name || 'NetworkError';
                onError.next(wrapped);
                throw wrapped;
            });
        return this.promise;
    }
}

function jsonRequest(method: string, url: string): Request {
    return new Request(method, url).set('Content-Type', 'application/json');
}

export default {
    setBaseHRef(val: string) {
        baseHRef = val;
    },
    toAbsURL,
    onError: onError.asObservable().pipe(filter(err => err != null)),
    get(url: string) {
        return new Request('GET', `${apiRoot()}${url}`);
    },
    getAbs(url: string) {
        return new Request('GET', url);
    },
    post(url: string) {
        return jsonRequest('POST', `${apiRoot()}${url}`);
    },
    put(url: string) {
        return jsonRequest('PUT', `${apiRoot()}${url}`);
    },
    patch(url: string) {
        return jsonRequest('PATCH', `${apiRoot()}${url}`);
    },
    delete(url: string) {
        return jsonRequest('DELETE', `${apiRoot()}${url}`);
    },
    loadEventSource(url: string): Observable<string> {
        return Observable.create((observer: Observer<any>) => {
            const fullUrl = `${apiRoot()}${url}`;

            const abortController = new AbortController();
            let errored = false;

            // EventSource's error event is opaque (no status, no body). On failure,
            // issue a one-shot fetch so we can surface the server's HTTP status and
            // body. This replaces the always-on prefetch, which doubled the open
            // watch streams per subscription and held an HTTP/1.1 connection slot
            // for the lifetime of every watch — see issue #27877.
            const emitError = (err: ResponseError) => {
                observer.error(err);
                onError.next(err);
            };
            const probeAndError = (eventSourceError: any) => {
                if (errored) {
                    return;
                }
                errored = true;
                fetch(fullUrl, {signal: abortController.signal})
                    .then(async response => {
                        if (!response.ok) {
                            const text = await response.text();
                            const apiResponse = buildApiResponse(response, text);
                            emitError(new ResponseError(errorMessageFor(apiResponse), response.status, apiResponse));
                        } else {
                            await response.body?.cancel();
                            const message = eventSourceError?.message || eventSourceError?.type || String(eventSourceError ?? 'EventSource error');
                            const err = new ResponseError(message, 0);
                            err.name = 'EventSourceError';
                            emitError(err);
                        }
                    })
                    .catch(fetchErr => {
                        if (fetchErr?.name === 'AbortError') {
                            return;
                        }
                        const err = new ResponseError(fetchErr?.message || String(fetchErr), 0);
                        err.name = fetchErr?.name || 'NetworkError';
                        emitError(err);
                    });
            };

            let eventSource = new EventSource(fullUrl);
            eventSource.onmessage = msg => observer.next(msg.data);
            eventSource.onerror = e => probeAndError(e);

            // EventSource does not provide easy way to get notification when connection closed.
            // check readyState periodically instead.
            const interval = setInterval(() => {
                if (eventSource && eventSource.readyState === ReadyState.CLOSED) {
                    probeAndError('connection got closed unexpectedly');
                }
            }, 500);
            return () => {
                clearInterval(interval);
                eventSource.close();
                abortController.abort();
                eventSource = null;
            };
        });
    }
};
