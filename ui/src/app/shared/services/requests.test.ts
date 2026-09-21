import {Subscription} from 'rxjs';

import requests from './requests';

class FakeEventSource {
    public static instances: FakeEventSource[] = [];
    public onopen: (data: any) => void;
    public onmessage: (data: any) => void;
    public onerror: (data: any) => void;
    public readyState = 0;
    public closed = false;

    constructor(public url: string) {
        FakeEventSource.instances.push(this);
    }

    public close() {
        this.closed = true;
    }
}

function setHidden(hidden: boolean) {
    Object.defineProperty(document, 'hidden', {configurable: true, value: hidden});
    document.dispatchEvent(new Event('visibilitychange'));
    // the visibility handler is debounced to ignore quick back and forth switches
    jest.advanceTimersByTime(600);
}

describe('loadEventSource', () => {
    let subscription: Subscription;

    beforeEach(() => {
        jest.useFakeTimers();
        FakeEventSource.instances = [];
        (global as any).EventSource = FakeEventSource;
        Object.defineProperty(document, 'hidden', {configurable: true, value: false});
    });

    afterEach(() => {
        subscription?.unsubscribe();
        jest.useRealTimers();
    });

    it('releases the connection while the tab is hidden and reconnects when it is shown again', () => {
        subscription = requests.loadEventSource('/stream/applications').subscribe();
        expect(FakeEventSource.instances).toHaveLength(1);

        setHidden(true);
        expect(FakeEventSource.instances[0].closed).toBe(true);
        expect(FakeEventSource.instances).toHaveLength(1);

        setHidden(false);
        expect(FakeEventSource.instances).toHaveLength(2);
        expect(FakeEventSource.instances[1].closed).toBe(false);
    });

    it('does not open a connection while the tab is hidden', () => {
        Object.defineProperty(document, 'hidden', {configurable: true, value: true});

        subscription = requests.loadEventSource('/stream/applications').subscribe();

        expect(FakeEventSource.instances).toHaveLength(0);
    });

    it('closes the connection when the subscription ends', () => {
        subscription = requests.loadEventSource('/stream/applications').subscribe();

        subscription.unsubscribe();

        expect(FakeEventSource.instances[0].closed).toBe(true);
    });
});
