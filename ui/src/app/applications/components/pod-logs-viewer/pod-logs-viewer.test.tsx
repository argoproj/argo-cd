import * as React from 'react';
import {act, render, screen} from '@testing-library/react';
import {MemoryRouter} from 'react-router-dom';
import {Subject} from 'rxjs';

import {LogEntry} from '../../../shared/models';
import {services} from '../../../shared/services';
import {PodsLogsViewer} from './pod-logs-viewer';

// AutoSizer measures its parent, which is always 0x0 in jsdom and would render nothing.
jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: any) => children({width: 800, height: 600})
}));

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getContainerLogs: jest.fn(),
            getDownloadLogsURL: jest.fn(() => '')
        },
        viewPreferences: {
            getPreferences: jest.fn(() => Promise.resolve({appDetails: {darkMode: false, wrapLines: false}}))
        }
    }
}));

const entry = (content: string, first?: boolean): LogEntry => ({
    content,
    podName: 'pod-1',
    timeStamp: null,
    timeStampStr: '',
    last: false,
    ...(first ? {first: true} : {})
});

const props = {
    namespace: 'default',
    applicationNamespace: 'argocd',
    applicationName: 'test-app',
    podName: 'pod-1',
    containerName: 'main'
};

describe('PodsLogsViewer reconnect handling', () => {
    let stream: Subject<LogEntry>;

    beforeEach(() => {
        jest.useFakeTimers();
        stream = new Subject<LogEntry>();
        (services.applications.getContainerLogs as jest.Mock).mockReturnValue(stream);
    });

    afterEach(() => {
        jest.useRealTimers();
        jest.clearAllMocks();
    });

    // bufferTime(100) batches entries, so emissions must be flushed before asserting.
    const emit = async (entries: LogEntry[]) => {
        await act(async () => {
            entries.forEach(e => stream.next(e));
            jest.advanceTimersByTime(150);
        });
    };

    const renderViewer = async () => {
        const view = render(
            <MemoryRouter>
                <PodsLogsViewer {...props} />
            </MemoryRouter>
        );
        // let the viewPreferences DataLoader resolve
        await act(async () => {
            await Promise.resolve();
        });
        return view;
    };

    it('appends entries while the connection stays up', async () => {
        await renderViewer();
        await emit([entry('line 1', true), entry('line 2'), entry('line 3')]);

        expect(screen.getByText('line 1')).toBeInTheDocument();
        expect(screen.getByText('line 3')).toBeInTheDocument();
    });

    it('drops the buffer when a reconnect replays the log', async () => {
        await renderViewer();
        await emit([entry('line 1', true), entry('line 2'), entry('line 3')]);

        // reconnect: the backend replays from the start, first entry marked `first`
        await emit([entry('line 1', true), entry('line 2'), entry('line 3'), entry('line 4')]);

        // each line must appear exactly once, not twice
        expect(screen.getAllByText('line 1')).toHaveLength(1);
        expect(screen.getAllByText('line 2')).toHaveLength(1);
        expect(screen.getAllByText('line 4')).toHaveLength(1);
    });

    it('truncates correctly when the restart marker lands mid-batch', async () => {
        await renderViewer();
        await emit([entry('line 1', true), entry('line 2')]);

        // bufferTime batched a trailing entry of the old connection with the reconnect
        await emit([entry('trailing'), entry('line 1', true), entry('line 2')]);

        expect(screen.queryByText('trailing')).not.toBeInTheDocument();
        expect(screen.getAllByText('line 1')).toHaveLength(1);
        expect(screen.getAllByText('line 2')).toHaveLength(1);
    });

    it('keeps streaming after a non-JSON stream error', async () => {
        // loadEventSource reports a dropped connection as a plain string, so the error
        // handler must not assume a JSON body nor surface the max-pods message.
        await renderViewer();
        await emit([entry('line 1', true)]);

        await act(async () => {
            stream.error('connection got closed unexpectedly');
            jest.advanceTimersByTime(600);
        });

        expect(screen.queryByText(/Max pods to view logs are reached/)).not.toBeInTheDocument();
        expect(screen.getByText('line 1')).toBeInTheDocument();
    });

    it('surfaces the max pods message instead of a blank view', async () => {
        await renderViewer();

        await act(async () => {
            stream.error({body: JSON.stringify({error: {message: 'max pods to view logs are reached. Please provide more granular query'}})});
            await Promise.resolve();
        });

        expect(screen.getByText('Max pods to view logs are reached. Please provide more granular query.')).toBeInTheDocument();
    });
});
