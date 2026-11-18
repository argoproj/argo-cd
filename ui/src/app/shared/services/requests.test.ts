import requests, {STREAM_STATUS_TIMEOUT} from './requests';

const CONNECTING = 0;
const OPEN = 1;
const CLOSED = 2;

class FakeEventSource {
    public static instances: FakeEventSource[] = [];
    public onmessage: (msg: {data: string}) => void;
    public onerror: (e: any) => void;
    public readyState = OPEN;
    public closed = false;

    constructor(public url: string) {
        FakeEventSource.instances.push(this);
    }

    public close() {
        this.closed = true;
        this.readyState = CLOSED;
    }

    // The connection dropped: the browser reconnects by itself unless the stream is closed.
    public drop() {
        this.readyState = CONNECTING;
        this.onerror({type: 'error'});
    }

    // The server answered with an error status: the browser gives up on the stream.
    public refuse() {
        this.readyState = CLOSED;
        this.onerror({type: 'error'});
    }
}

const flushPromises = async () => {
    for (let i = 0; i < 5; i++) {
        await Promise.resolve();
    }
};

describe('loadEventSource', () => {
    let fetchMock: jest.Mock;
    let next: jest.Mock;
    let error: jest.Mock;
    let reported: jest.Mock;
    let source: FakeEventSource;
    let subscription: {unsubscribe(): void};
    let reportedSubscription: {unsubscribe(): void};

    // A probe that the server accepted but has not answered, as a watch with nothing to send does.
    const pendingProbe = () => fetchMock.mockReturnValue(new Promise(() => undefined));

    beforeEach(() => {
        jest.useFakeTimers();
        FakeEventSource.instances = [];
        (globalThis as any).EventSource = FakeEventSource;
        fetchMock = jest.fn();
        (globalThis as any).fetch = fetchMock;
        next = jest.fn();
        error = jest.fn();
        reported = jest.fn();
        // onError replays the last error to new subscribers; only look at the ones of this test.
        let subscribed = false;
        reportedSubscription = requests.onError.subscribe(err => subscribed && reported(err));
        subscribed = true;
        subscription = requests.loadEventSource('/stream/applications?resourceVersion=1').subscribe({next, error});
        source = FakeEventSource.instances[0];
    });

    afterEach(() => {
        subscription.unsubscribe();
        reportedSubscription.unsubscribe();
        jest.useRealTimers();
        delete (globalThis as any).EventSource;
        delete (globalThis as any).fetch;
    });

    it('opens the stream and emits its messages', () => {
        expect(source.url).toBe('/api/v1/stream/applications?resourceVersion=1');

        source.onmessage({data: 'first'});
        source.onmessage({data: 'second'});

        expect(next.mock.calls).toEqual([['first'], ['second']]);
        expect(error).not.toHaveBeenCalled();
    });

    describe('when the connection drops', () => {
        it('errors at once, without probing for a status there is none of', () => {
            // A probe is the same stream again. Waiting for it leaves nothing listening until the next
            // change, which the probe then takes for itself.
            pendingProbe();

            source.drop();

            expect(error).toHaveBeenCalledWith({type: 'error'});
            expect(reported).toHaveBeenCalledWith({type: 'error'});
            expect(fetchMock).not.toHaveBeenCalled();
        });

        it('closes the stream, so that the browser does not reconnect it next to the subscriber', () => {
            source.drop();

            expect(source.closed).toBe(true);
        });
    });

    describe('when the server refuses the stream', () => {
        it('errors with the status and body of the response', async () => {
            fetchMock.mockResolvedValue({ok: false, status: 403, statusText: 'Forbidden', text: () => Promise.resolve('{"error":"permission denied"}')});

            source.refuse();
            expect(error).not.toHaveBeenCalled();
            await flushPromises();

            expect(fetchMock).toHaveBeenCalledWith('/api/v1/stream/applications?resourceVersion=1', expect.objectContaining({signal: expect.anything()}));
            expect(error).toHaveBeenCalledWith({status: 403, statusText: 'Forbidden', body: '{"error":"permission denied"}'});
            expect(reported).toHaveBeenCalledWith({status: 403, name: 'Forbidden', message: '{"error":"permission denied"}'});
        });

        it('errors with the stream error when the probe is accepted, and closes the probe', async () => {
            const cancel = jest.fn().mockResolvedValue(undefined);
            fetchMock.mockResolvedValue({ok: true, status: 200, body: {cancel}});

            source.refuse();
            await flushPromises();

            expect(cancel).toHaveBeenCalled();
            expect(error).toHaveBeenCalledWith({type: 'error'});
            expect(reported).toHaveBeenCalledWith({type: 'error'});
        });

        it('errors with the failure of the probe', async () => {
            const failure = new TypeError('Failed to fetch');
            fetchMock.mockRejectedValue(failure);

            source.refuse();
            await flushPromises();

            expect(error).toHaveBeenCalledWith(failure);
            expect(reported).toHaveBeenCalledWith(failure);
        });

        it('stops waiting for a probe that is not answered', async () => {
            pendingProbe();

            source.refuse();
            await flushPromises();
            jest.advanceTimersByTime(STREAM_STATUS_TIMEOUT - 1);
            expect(error).not.toHaveBeenCalled();

            jest.advanceTimersByTime(1);

            expect(error).toHaveBeenCalledTimes(1);
            expect(error).toHaveBeenCalledWith({type: 'error'});
            expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true);
        });

        it('errors once when the probe is answered after all', async () => {
            let answer: (response: any) => void;
            fetchMock.mockReturnValue(new Promise(resolve => (answer = resolve)));

            source.refuse();
            jest.advanceTimersByTime(STREAM_STATUS_TIMEOUT);
            answer({ok: false, status: 504, statusText: 'Gateway Timeout', text: () => Promise.resolve('')});
            await flushPromises();

            expect(error).toHaveBeenCalledTimes(1);
            expect(error).toHaveBeenCalledWith({type: 'error'});
            expect(reported).toHaveBeenCalledTimes(1);
            expect(reported).toHaveBeenCalledWith({type: 'error'});
        });

        it('notices a stream that closed without an error event', async () => {
            fetchMock.mockResolvedValue({ok: true, status: 200, body: {cancel: () => Promise.resolve()}});

            source.readyState = CLOSED;
            jest.advanceTimersByTime(500);
            await flushPromises();

            expect(error).toHaveBeenCalledWith('connection got closed unexpectedly');
        });
    });

    it('closes the stream and its probe when unsubscribed', async () => {
        pendingProbe();
        source.refuse();

        subscription.unsubscribe();
        jest.advanceTimersByTime(STREAM_STATUS_TIMEOUT);
        await flushPromises();

        expect(source.closed).toBe(true);
        expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true);
        expect(error).not.toHaveBeenCalled();
        expect(reported).not.toHaveBeenCalled();
    });
});
