import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {Provider} from '../../../shared/context';
import {PodDebugViewer} from './pod-debug-viewer';

// xterm 4 needs a 2D canvas context in Terminal.open(), which jsdom does not provide.
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

class FakeWebSocket {
    static instances: FakeWebSocket[] = [];
    closed = false;
    sent: string[] = [];
    onopen: (() => void) | null = null;
    onclose: (() => void) | null = null;
    onerror: ((e?: any) => void) | null = null;
    onmessage: ((e: {data: string}) => void) | null = null;
    constructor(public url: string) {
        FakeWebSocket.instances.push(this);
    }
    send(data: string) {
        this.sent.push(data);
    }
    close() {
        this.closed = true;
    }
}

const podState: any = {
    metadata: {name: 'web-0', namespace: 'cs2'},
    spec: {containers: [{name: 'apache'}]}
};
const selectedNode: any = {name: 'web-0', namespace: 'cs2', kind: 'Pod', group: '', version: 'v1', uid: 'uid'};
const showNotification = jest.fn();

const Viewer = () => (
    <Provider value={{notifications: {show: showNotification}, popup: {}, navigation: {}, baseHref: '/', history: {}} as any}>
        <PodDebugViewer
            applicationName='demo'
            applicationNamespace='argocd'
            projectName='default'
            selectedNode={selectedNode}
            podState={podState}
            debugImages={['busybox:1.36']}
        />
    </Provider>
);

describe('PodDebugViewer', () => {
    const realWebSocket = (globalThis as any).WebSocket;

    beforeEach(() => {
        FakeWebSocket.instances.length = 0;
        mockTerminals.length = 0;
        showNotification.mockClear();
        (globalThis as any).WebSocket = FakeWebSocket;
    });

    afterEach(() => {
        (globalThis as any).WebSocket = realWebSocket;
    });

    it('keeps the session socket open after Start and renders its output', () => {
        render(<Viewer />);
        fireEvent.click(screen.getByText('Start Debug Session', {selector: 'button'}));

        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(FakeWebSocket.instances[0].closed).toBe(false);
        expect(new URL(FakeWebSocket.instances[0].url).searchParams.get('image')).toBe('busybox:1.36');
        expect(mockTerminals).toHaveLength(1);
        expect(mockTerminals[0].disposed).toBe(false);

        act(() => FakeWebSocket.instances[0].onopen?.());
        act(() => FakeWebSocket.instances[0].onmessage?.({data: JSON.stringify({operation: 'stdout', data: '/ # '})}));
        expect(FakeWebSocket.instances[0].closed).toBe(false);
        expect(mockTerminals[0].written).toEqual(['/ # ']);
    });

    it('closes the socket on End Session', () => {
        render(<Viewer />);
        fireEvent.click(screen.getByText('Start Debug Session', {selector: 'button'}));
        const [socket] = FakeWebSocket.instances;

        fireEvent.click(screen.getByText('End Session'));

        expect(socket.closed).toBe(true);
        expect(mockTerminals[0].disposed).toBe(true);
        expect(screen.getByText('Start Debug Session', {selector: 'button'})).toBeInTheDocument();
    });

    it('ends the session instead of reconnecting on a token refresh frame', () => {
        render(<Viewer />);
        fireEvent.click(screen.getByText('Start Debug Session', {selector: 'button'}));
        const [socket] = FakeWebSocket.instances;
        act(() => socket.onopen?.());

        act(() => socket.onmessage?.({data: JSON.stringify({Code: 1})}));

        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(socket.closed).toBe(true);
        expect(showNotification).toHaveBeenCalledTimes(1);
        expect(screen.getByText('Start Debug Session', {selector: 'button'})).toBeInTheDocument();
    });
});
