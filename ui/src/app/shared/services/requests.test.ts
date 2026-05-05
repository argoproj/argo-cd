import requests, {ResponseError} from './requests';

const flushMicrotasks = () => new Promise(resolve => setTimeout(resolve, 0));

describe('Request', () => {
    let originalFetch: any;

    beforeEach(() => {
        originalFetch = (global as any).fetch;
    });

    afterEach(() => {
        (global as any).fetch = originalFetch;
    });

    const jsonResponse = (status: number, body: any, ok = status >= 200 && status < 300) => {
        const text = typeof body === 'string' ? body : JSON.stringify(body);
        return {
            ok,
            status,
            statusText: ok ? 'OK' : 'Error',
            headers: {get: (name: string) => (name.toLowerCase() === 'content-type' ? 'application/json' : null)},
            text: () => Promise.resolve(text)
        };
    };

    test('serializes object query params with array values into a repeated query string', async () => {
        const fetchSpy = jest.fn().mockResolvedValue(jsonResponse(200, {ok: true}));
        (global as any).fetch = fetchSpy;

        await requests.get('/foo').query({a: 1, b: ['x', 'y'], skip: null});

        expect(fetchSpy).toHaveBeenCalledTimes(1);
        const url = fetchSpy.mock.calls[0][0] as string;
        expect(url).toContain('/foo?');
        const qs = url.split('?')[1];
        const params = new URLSearchParams(qs);
        expect(params.getAll('a')).toEqual(['1']);
        expect(params.getAll('b')).toEqual(['x', 'y']);
        expect(params.has('skip')).toBe(false);
    });

    test('parses JSON response bodies into res.body', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue(jsonResponse(200, {hello: 'world'}));

        const res = await requests.get('/foo');

        expect(res.status).toBe(200);
        expect(res.body).toEqual({hello: 'world'});
    });

    test('rejects with a ResponseError carrying status and parsed body on non-2xx', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue(jsonResponse(404, {message: 'not found'}, false));

        let caught: any;
        try {
            await requests.get('/missing');
        } catch (e) {
            caught = e;
        }

        expect(caught).toBeInstanceOf(ResponseError);
        expect(caught.status).toBe(404);
        expect(caught.response.body).toEqual({message: 'not found'});
    });

    test('uses body.message as the ResponseError message when the JSON error body has one', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue(jsonResponse(404, {message: 'app not found'}, false));

        let caught: any;
        try {
            await requests.get('/missing');
        } catch (e) {
            caught = e;
        }

        expect(caught).toBeInstanceOf(ResponseError);
        expect(caught.message).toBe('app not found');
        expect(caught.response.body).toEqual({message: 'app not found'});
    });

    test('falls back to body.error when no body.message is present', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue(jsonResponse(500, {error: 'internal boom'}, false));

        let caught: any;
        try {
            await requests.get('/boom');
        } catch (e) {
            caught = e;
        }

        expect(caught).toBeInstanceOf(ResponseError);
        expect(caught.message).toBe('internal boom');
    });

    test('falls back to raw text when the JSON body has no message or error field', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue(jsonResponse(400, {details: 'something'}, false));

        let caught: any;
        try {
            await requests.get('/foo');
        } catch (e) {
            caught = e;
        }

        expect(caught).toBeInstanceOf(ResponseError);
        expect(caught.message).toBe('{"details":"something"}');
    });

    test('wraps network failures as a ResponseError with status 0', async () => {
        (global as any).fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'));

        let caught: any;
        try {
            await requests.get('/foo');
        } catch (e) {
            caught = e;
        }

        expect(caught).toBeInstanceOf(ResponseError);
        expect(caught.status).toBe(0);
        expect(caught.message).toBe('Failed to fetch');
    });
});

describe('loadEventSource', () => {
    let originalFetch: any;
    let originalEventSource: any;
    let createdEventSource: any;

    beforeEach(() => {
        originalFetch = (global as any).fetch;
        originalEventSource = (global as any).EventSource;
        createdEventSource = null;
        (global as any).EventSource = class {
            public onopen: any;
            public onmessage: any;
            public onerror: any;
            public readyState = 1;
            public closed = false;
            constructor(public url: string) {
                createdEventSource = this;
            }
            close() {
                this.closed = true;
                this.readyState = 2;
            }
        };
    });

    afterEach(() => {
        (global as any).fetch = originalFetch;
        (global as any).EventSource = originalEventSource;
    });

    test('does not prefetch on the happy path — only the EventSource connection is opened', async () => {
        const fetchSpy = jest.fn();
        (global as any).fetch = fetchSpy;

        const sub = requests.loadEventSource('/test').subscribe();
        await flushMicrotasks();

        expect(createdEventSource).not.toBeNull();
        expect(fetchSpy).not.toHaveBeenCalled();

        sub.unsubscribe();
    });

    test('probes with a one-shot fetch when EventSource errors, surfacing the HTTP status and body', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue({
            ok: false,
            status: 500,
            statusText: 'Internal Server Error',
            text: () => Promise.resolve('boom')
        });

        const errors: any[] = [];
        const sub = requests.loadEventSource('/test').subscribe({
            next: () => undefined,
            error: e => errors.push(e)
        });

        createdEventSource.onerror({type: 'error'});
        await flushMicrotasks();

        expect((global as any).fetch).toHaveBeenCalledTimes(1);
        expect(errors).toHaveLength(1);
        expect(errors[0]).toBeInstanceOf(ResponseError);
        expect(errors[0].status).toBe(500);
        expect(errors[0].response.body).toBe('boom');
        expect(errors[0].message).toBe('boom');

        sub.unsubscribe();
    });

    test('wraps the original EventSource error as a ResponseError when the probe sees a 2xx response', async () => {
        const cancelSpy = jest.fn().mockResolvedValue(undefined);
        (global as any).fetch = jest.fn().mockResolvedValue({
            ok: true,
            status: 200,
            body: {cancel: cancelSpy}
        });

        const errors: ResponseError[] = [];
        const sub = requests.loadEventSource('/test').subscribe({
            next: () => undefined,
            error: e => errors.push(e)
        });

        createdEventSource.onerror({type: 'error', message: 'connection lost'});
        await flushMicrotasks();

        expect((global as any).fetch).toHaveBeenCalledTimes(1);
        expect(cancelSpy).toHaveBeenCalledTimes(1);
        expect(errors).toHaveLength(1);
        expect(errors[0]).toBeInstanceOf(ResponseError);
        expect(errors[0].status).toBe(0);
        expect(errors[0].name).toBe('EventSourceError');
        expect(errors[0].message).toBe('connection lost');

        sub.unsubscribe();
    });

    test('coalesces repeated EventSource errors so only one probe is issued', async () => {
        (global as any).fetch = jest.fn().mockResolvedValue({
            ok: false,
            status: 503,
            statusText: 'Service Unavailable',
            text: () => Promise.resolve('down')
        });

        const errors: any[] = [];
        const sub = requests.loadEventSource('/test').subscribe({
            next: () => undefined,
            error: e => errors.push(e)
        });

        createdEventSource.onerror({type: 'error'});
        createdEventSource.onerror({type: 'error'});
        createdEventSource.onerror({type: 'error'});
        await flushMicrotasks();

        expect((global as any).fetch).toHaveBeenCalledTimes(1);
        expect(errors).toHaveLength(1);

        sub.unsubscribe();
    });
});
