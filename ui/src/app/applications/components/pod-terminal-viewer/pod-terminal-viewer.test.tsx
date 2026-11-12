import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {Provider} from '../../../shared/context';
import {PodTerminalViewer, PodTerminalViewerProps} from './pod-terminal-viewer';

// xterm 4 needs a 2D canvas context in Terminal.open(), which jsdom does not provide,
// so the terminal is stubbed. Instances are collected to assert dispose() and write() calls.
const mockTerminals: any[] = [];
jest.mock('xterm', () => ({
    Terminal: class {
        options: any = {};
        rows = 24;
        cols = 80;
        disposed = false;
        written: string[] = [];
        constructor() {
            mockTerminals.push(this);
        }
        open() {}
        loadAddon() {}
        onResize() {}
        onKey() {}
        onData() {}
        write(data: string) {
            this.written.push(data);
        }
        focus() {}
        dispose() {
            this.disposed = true;
        }
    }
}));
jest.mock('xterm-addon-fit', () => ({
    FitAddon: class {
        fit() {}
    }
}));

// jsdom ships a real WebSocket that would try to connect; replace it with a recorder that mimics
// the browser's event timing without timers: close() only moves the socket to CLOSING, and the
// events the browser dispatches afterwards (error then close for a socket that never finished
// connecting, close for an open one) stay queued until flush() delivers them, to whatever handler
// is attached at that moment. A handler detached before the flush therefore never runs.
class FakeWebSocket {
    static instances: FakeWebSocket[] = [];
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    private static deferred: Array<() => void> = [];
    readyState = FakeWebSocket.CONNECTING;
    closeCalls = 0;
    sent: string[] = [];
    onopen: ((e?: any) => void) | null = null;
    onclose: ((e?: any) => void) | null = null;
    onerror: ((e?: any) => void) | null = null;
    onmessage: ((e: {data: string}) => void) | null = null;
    constructor(public url: string) {
        FakeWebSocket.instances.push(this);
    }
    // Delivers every queued event in order, including the ones queued by the handlers it runs.
    static flush() {
        for (let dispatch = FakeWebSocket.deferred.shift(); dispatch; dispatch = FakeWebSocket.deferred.shift()) {
            dispatch();
        }
    }
    static reset() {
        FakeWebSocket.instances.length = 0;
        FakeWebSocket.deferred.length = 0;
    }
    get container() {
        return new URL(this.url).searchParams.get('container');
    }
    get handlersDetached() {
        return this.onopen === null && this.onclose === null && this.onerror === null && this.onmessage === null;
    }
    send(data: string) {
        this.sent.push(data);
    }
    // Like the browser, close() on a socket that is already closing or closed does nothing.
    close() {
        if (this.readyState >= FakeWebSocket.CLOSING) {
            return;
        }
        this.closeCalls++;
        const wasConnecting = this.readyState === FakeWebSocket.CONNECTING;
        this.readyState = FakeWebSocket.CLOSING;
        FakeWebSocket.deferred.push(() => {
            this.readyState = FakeWebSocket.CLOSED;
            if (wasConnecting) {
                this.onerror?.(new Event('error'));
            }
            this.onclose?.(new Event('close'));
        });
    }
    simulateOpen() {
        this.readyState = FakeWebSocket.OPEN;
        this.onopen?.(new Event('open'));
    }
    // Like the browser, a server message is only delivered while the socket is open.
    simulateMessage(data: string) {
        if (this.readyState === FakeWebSocket.OPEN) {
            this.onmessage?.({data});
        }
    }
}

const podState: any = {
    metadata: {name: 'web-0', namespace: 'cs2'},
    spec: {containers: [{name: 'apache'}, {name: 'php'}], initContainers: []},
    status: {
        containerStatuses: [
            {name: 'apache', state: {running: {}}},
            {name: 'php', state: {running: {}}}
        ]
    }
};
const selectedNode: any = {name: 'web-0', namespace: 'cs2', kind: 'Pod', group: '', version: 'v1', uid: 'uid'};
const showNotification = jest.fn();
const stdout = (data: string) => JSON.stringify({operation: 'stdout', data});
const tokenRefresh = JSON.stringify({Code: 1});

const Viewer = ({containerName, onClickContainer}: Pick<PodTerminalViewerProps, 'containerName' | 'onClickContainer'>) => (
    <Provider value={{notifications: {show: showNotification}, popup: {}, navigation: {}, baseHref: '/', history: {}} as any}>
        <PodTerminalViewer
            applicationName='demo'
            applicationNamespace='argocd'
            projectName='default'
            selectedNode={selectedNode}
            podState={podState}
            containerName={containerName}
            onClickContainer={onClickContainer}
        />
    </Provider>
);

// Mirrors resource-details.tsx: the parent owns the active container index and
// derives containerName from it, so a click re-renders the viewer with a new prop.
const Harness = () => {
    const [index, setIndex] = React.useState(0);
    const containers = podState.spec.containers.concat(podState.spec.initContainers);
    return <Viewer containerName={containers[index].name} onClickContainer={(group: any, i: number) => setIndex(group.offset + i)} />;
};

describe('PodTerminalViewer', () => {
    const realWebSocket = (globalThis as any).WebSocket;

    beforeEach(() => {
        FakeWebSocket.reset();
        mockTerminals.length = 0;
        showNotification.mockClear();
        (globalThis as any).WebSocket = FakeWebSocket;
    });

    afterEach(() => {
        (globalThis as any).WebSocket = realWebSocket;
    });

    it('opens a socket for the first container, sends the initial resize once connected and writes server output', () => {
        render(<Harness />);
        expect(FakeWebSocket.instances.map(s => s.container)).toEqual(['apache']);
        const [apache] = FakeWebSocket.instances;
        act(() => apache.simulateOpen());
        expect(apache.sent).toHaveLength(1);
        expect(JSON.parse(apache.sent[0]).operation).toBe('resize');
        expect(mockTerminals).toHaveLength(1);

        act(() => apache.simulateMessage(stdout('apache $ ')));
        expect(mockTerminals[0].written).toEqual(['apache $ ']);
    });

    it('switching container closes the previous socket and keeps the new one alive', () => {
        render(<Harness />);
        const [apache] = FakeWebSocket.instances;
        act(() => apache.simulateOpen());

        fireEvent.click(screen.getByText('php'));
        // The browser now delivers the deferred close/error events of whatever the switch closed.
        act(() => FakeWebSocket.flush());

        expect(FakeWebSocket.instances.map(s => s.container)).toEqual(['apache', 'php']);
        const php = FakeWebSocket.instances[1];
        // The new socket must survive the switch: it was still connecting when the previous effect's
        // cleanup ran, and closing it there (error toast, then a dead terminal) is the bug this test pins.
        expect(showNotification).not.toHaveBeenCalled();
        expect(php.readyState).toBe(FakeWebSocket.CONNECTING);
        expect(php.closeCalls).toBe(0);
        expect(php.handlersDetached).toBe(false);
        expect(mockTerminals).toHaveLength(2);
        expect(mockTerminals[1].disposed).toBe(false);
        expect(apache.readyState).toBe(FakeWebSocket.CLOSED);
        expect(apache.closeCalls).toBe(1);
        expect(apache.handlersDetached).toBe(true);
        expect(mockTerminals[0].disposed).toBe(true);
        expect(screen.getByTitle('php').querySelector('i.pod-terminal-viewer__icon')).not.toBeNull();

        act(() => php.simulateOpen());
        expect(php.sent).toHaveLength(1);
        act(() => php.simulateMessage(stdout('php $ ')));
        expect(mockTerminals[1].written).toEqual(['php $ ']);
        expect(php.readyState).toBe(FakeWebSocket.OPEN);
        expect(php.closeCalls).toBe(0);
    });

    it('a containerName prop change without a click switches the session the same way', () => {
        const {rerender} = render(<Viewer containerName='apache' />);
        const [apache] = FakeWebSocket.instances;
        act(() => apache.simulateOpen());

        rerender(<Viewer containerName='php' />);
        act(() => FakeWebSocket.flush());

        expect(FakeWebSocket.instances.map(s => s.container)).toEqual(['apache', 'php']);
        const php = FakeWebSocket.instances[1];
        expect(showNotification).not.toHaveBeenCalled();
        expect(php.readyState).toBe(FakeWebSocket.CONNECTING);
        expect(php.closeCalls).toBe(0);
        expect(mockTerminals.map(t => t.disposed)).toEqual([true, false]);
        expect(apache.readyState).toBe(FakeWebSocket.CLOSED);
        expect(apache.closeCalls).toBe(1);
        expect(apache.handlersDetached).toBe(true);

        act(() => php.simulateOpen());
        expect(php.sent).toHaveLength(1);
        act(() => php.simulateMessage(stdout('php $ ')));
        expect(mockTerminals[1].written).toEqual(['php $ ']);
        expect(php.closeCalls).toBe(0);
    });

    it('switching while the first socket is still connecting closes it silently', () => {
        render(<Harness />);
        const [apache] = FakeWebSocket.instances;

        fireEvent.click(screen.getByText('php'));
        // A socket closed while connecting gets an error event from the browser; it must not reach the toast.
        act(() => FakeWebSocket.flush());

        const php = FakeWebSocket.instances[1];
        expect(showNotification).not.toHaveBeenCalled();
        expect(php.readyState).toBe(FakeWebSocket.CONNECTING);
        expect(php.closeCalls).toBe(0);
        expect(mockTerminals[1].disposed).toBe(false);
        expect(apache.readyState).toBe(FakeWebSocket.CLOSED);
        expect(apache.closeCalls).toBe(1);
        expect(apache.handlersDetached).toBe(true);

        act(() => php.simulateOpen());
        act(() => php.simulateMessage(stdout('php $ ')));
        expect(mockTerminals[1].written).toEqual(['php $ ']);
        expect(php.closeCalls).toBe(0);
    });

    it('rapid double switch ends with one live socket for the last container', () => {
        render(<Harness />);
        fireEvent.click(screen.getByText('php'));
        fireEvent.click(screen.getByText('apache'));
        act(() => FakeWebSocket.flush());

        expect(showNotification).not.toHaveBeenCalled();
        expect(FakeWebSocket.instances.map(s => s.container)).toEqual(['apache', 'php', 'apache']);
        expect(FakeWebSocket.instances.map(s => s.readyState)).toEqual([FakeWebSocket.CLOSED, FakeWebSocket.CLOSED, FakeWebSocket.CONNECTING]);
        expect(FakeWebSocket.instances.map(s => s.closeCalls)).toEqual([1, 1, 0]);
        expect(FakeWebSocket.instances.map(s => s.handlersDetached)).toEqual([true, true, false]);
        expect(mockTerminals.map(t => t.disposed)).toEqual([true, true, false]);

        const last = FakeWebSocket.instances[2];
        act(() => last.simulateOpen());
        expect(last.sent).toHaveLength(1);
        act(() => last.simulateMessage(stdout('apache $ ')));
        expect(mockTerminals[2].written).toEqual(['apache $ ']);
        expect(last.closeCalls).toBe(0);
    });

    it('clicking the already-selected container does nothing', () => {
        render(<Harness />);
        const [socket] = FakeWebSocket.instances;
        act(() => socket.simulateOpen());

        fireEvent.click(screen.getByText('apache'));
        act(() => FakeWebSocket.flush());

        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(socket.readyState).toBe(FakeWebSocket.OPEN);
        expect(socket.closeCalls).toBe(0);
        expect(mockTerminals).toHaveLength(1);
        expect(mockTerminals[0].disposed).toBe(false);
        expect(showNotification).not.toHaveBeenCalled();
    });

    it('unmount closes the current socket exactly once without a toast', () => {
        const {unmount} = render(<Harness />);
        const [socket] = FakeWebSocket.instances;
        act(() => socket.simulateOpen());

        unmount();
        act(() => FakeWebSocket.flush());

        expect(showNotification).not.toHaveBeenCalled();
        expect(socket.readyState).toBe(FakeWebSocket.CLOSED);
        expect(socket.closeCalls).toBe(1);
        expect(socket.handlersDetached).toBe(true);
        expect(mockTerminals[0].disposed).toBe(true);
    });

    it('unmount while still connecting closes the socket without a toast', () => {
        const {unmount} = render(<Harness />);
        const [socket] = FakeWebSocket.instances;

        unmount();
        act(() => FakeWebSocket.flush());

        expect(showNotification).not.toHaveBeenCalled();
        expect(socket.readyState).toBe(FakeWebSocket.CLOSED);
        expect(socket.closeCalls).toBe(1);
        expect(socket.handlersDetached).toBe(true);
    });

    it('a token-refresh frame reconnects in place and keeps the terminal', () => {
        const {unmount} = render(<Harness />);
        const [first] = FakeWebSocket.instances;
        act(() => first.simulateOpen());

        act(() => first.simulateMessage(tokenRefresh));
        // The old socket's deferred close arrives while the replacement is still connecting.
        act(() => FakeWebSocket.flush());

        expect(FakeWebSocket.instances.map(s => s.container)).toEqual(['apache', 'apache']);
        const second = FakeWebSocket.instances[1];
        expect(showNotification).not.toHaveBeenCalled();
        expect(second.readyState).toBe(FakeWebSocket.CONNECTING);
        expect(second.closeCalls).toBe(0);
        expect(second.handlersDetached).toBe(false);
        expect(first.readyState).toBe(FakeWebSocket.CLOSED);
        expect(first.closeCalls).toBe(1);
        expect(first.handlersDetached).toBe(true);
        expect(mockTerminals).toHaveLength(1);
        expect(mockTerminals[0].disposed).toBe(false);

        act(() => second.simulateOpen());
        expect(second.sent).toHaveLength(1);
        act(() => second.simulateMessage(stdout('still here')));
        expect(mockTerminals[0].written).toEqual(['still here']);

        // The reconnected socket, not the one captured at mount, is what unmount closes.
        unmount();
        act(() => FakeWebSocket.flush());
        expect(showNotification).not.toHaveBeenCalled();
        expect(second.readyState).toBe(FakeWebSocket.CLOSED);
        expect(second.closeCalls).toBe(1);
        expect(first.closeCalls).toBe(1);
    });

    it('the old socket closing after the token-refresh replacement has opened leaves the replacement connected', () => {
        render(<Harness />);
        const [first] = FakeWebSocket.instances;
        act(() => first.simulateOpen());

        act(() => first.simulateMessage(tokenRefresh));
        const second = FakeWebSocket.instances[1];
        act(() => second.simulateOpen());
        // Only now does the browser report the old socket closed (a slow close handshake).
        act(() => FakeWebSocket.flush());

        expect(showNotification).not.toHaveBeenCalled();
        expect(second.readyState).toBe(FakeWebSocket.OPEN);
        expect(second.closeCalls).toBe(0);
        expect(first.readyState).toBe(FakeWebSocket.CLOSED);
        act(() => second.simulateMessage(stdout('still here')));
        expect(mockTerminals[0].written).toEqual(['still here']);
    });

    it('unmount right after a token-refresh reconnect closes the still-connecting replacement', () => {
        const {unmount} = render(<Harness />);
        const [first] = FakeWebSocket.instances;
        act(() => first.simulateOpen());
        act(() => first.simulateMessage(tokenRefresh));
        const second = FakeWebSocket.instances[1];

        unmount();
        act(() => FakeWebSocket.flush());

        expect(showNotification).not.toHaveBeenCalled();
        expect(second.readyState).toBe(FakeWebSocket.CLOSED);
        expect(second.closeCalls).toBe(1);
        expect(second.handlersDetached).toBe(true);
    });

    it('an error on the live socket shows the toast once and closes it', () => {
        render(<Harness />);
        const [socket] = FakeWebSocket.instances;
        act(() => socket.simulateOpen());

        act(() => socket.onerror(new Event('error')));
        act(() => FakeWebSocket.flush());

        expect(showNotification).toHaveBeenCalledTimes(1);
        expect(socket.readyState).toBe(FakeWebSocket.CLOSED);
        expect(socket.closeCalls).toBe(1);
    });
});
