import * as React from 'react';
import {act} from 'react';
import {createRoot, Root} from 'react-dom/client';
import {EMPTY, of, Subject} from 'rxjs';
import {LogEntry} from '../../../shared/models';
import {PodsLogsViewer} from './pod-logs-viewer';

const mockGetContainerLogs = jest.fn();
const mockCopyLogsButton = jest.fn();

jest.mock('argo-ui', () => ({
    DataLoader: ({children}: {children: (data: any) => React.ReactNode}) => children({appDetails: {darkMode: false, wrapLines: false}}),
    Tooltip: ({children, content}: {children: React.ReactNode; content: string}) => {
        const React = require('react');
        return React.createElement('span', {'data-tooltip-content': content}, children);
    }
}));

jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: ({width, height}: {width: number; height: number}) => React.ReactNode}) => children({width: 800, height: 400})
}));

jest.mock('ansi-to-react', () => ({
    __esModule: true,
    default: ({children}: {children: React.ReactNode}) => {
        const React = require('react');
        return React.createElement(React.Fragment, null, children);
    }
}));

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getContainerLogs: (...args: any[]) => mockGetContainerLogs(...args)
        },
        viewPreferences: {
            getPreferences: jest.fn()
        }
    }
}));

jest.mock('./copy-logs-button', () => ({
    CopyLogsButton: ({logs}: {logs: LogEntry[]}) => {
        mockCopyLogsButton(logs);
        return null;
    }
}));
jest.mock('./download-logs-button', () => ({DownloadLogsButton: () => null}));
jest.mock('./container-selector', () => ({ContainerSelector: () => null}));
jest.mock('./follow-toggle-button', () => ({FollowToggleButton: () => null}));
jest.mock('./show-previous-logs-toggle-button', () => ({ShowPreviousLogsToggleButton: () => null}));
jest.mock('./pod-logs-highlight-button', () => ({PodHighlightButton: () => null}));
jest.mock('./timestamps-toggle-button', () => ({TimestampsToggleButton: () => null}));
jest.mock('./dark-mode-toggle-button', () => ({DarkModeToggleButton: () => null}));
jest.mock('./fullscreen-button', () => ({FullscreenButton: () => null}));
jest.mock('./log-message-filter', () => ({LogMessageFilter: () => null}));
jest.mock('./since-seconds-selector', () => ({SinceSecondsSelector: () => null}));
jest.mock('./tail-selector', () => ({TailSelector: () => null}));
jest.mock('./pod-names-toggle-button', () => ({PodNamesToggleButton: () => null}));
jest.mock('./auto-scroll-button', () => ({AutoScrollButton: () => null}));
jest.mock('./wrap-lines-button', () => ({WrapLinesButton: () => null}));
jest.mock('./match-case-toggle-button', () => ({MatchCaseToggleButton: () => null}));

const logsFixture: LogEntry[] = [
    {
        content: 'INFO  Starting application',
        last: false,
        podName: 'demo-app-68b8fcf645-pkc5s',
        timeStamp: new Date().toISOString(),
        timeStampStr: '2026-04-03T19:00:00.000Z'
    },
    {
        content: 'INFO  Listening on :8080',
        last: false,
        podName: 'demo-app-68b8fcf645-pkc5s',
        timeStamp: new Date().toISOString(),
        timeStampStr: '2026-04-03T19:00:01.000Z'
    }
];

const makeComponent = () => (
    <PodsLogsViewer
        applicationName='demo-app'
        applicationNamespace='argocd'
        namespace='default'
        containerName='glady-app'
        podName='demo-app-68b8fcf645-pkc5s'
    />
);

describe('PodsLogsViewer clear logs button', () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        jest.clearAllMocks();
        container = document.createElement('div');
        document.body.appendChild(container);
        root = createRoot(container);
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
    });

    const renderComponent = () => {
        act(() => {
            root.render(makeComponent());
        });
    };

    const getClearButton = () => {
        const clearButtonIcon = container.querySelector('.fa-eraser');
        expect(clearButtonIcon).toBeTruthy();
        return clearButtonIcon.closest('button') as HTMLButtonElement;
    };

    it('keeps the clear button disabled when no log has been loaded and ignores clicks', () => {
        mockGetContainerLogs.mockReturnValue(EMPTY);

        renderComponent();

        const clearButton = getClearButton();
        expect(clearButton).toHaveClass('disabled');
        expect(clearButton).not.toBeDisabled();
        expect(clearButton.closest('[data-tooltip-content]')).toHaveAttribute('data-tooltip-content', 'Clear displayed logs');

        const beforeClick = container.textContent;
        act(() => {
            clearButton.click();
        });

        expect(container.textContent).toBe(beforeClick);
    });

    it('clears displayed logs but keeps received logs available for copying', () => {
        mockGetContainerLogs.mockReturnValue(of(...logsFixture));

        renderComponent();

        const beforeClear = container.textContent;
        expect(beforeClear).toContain('INFO  Starting application');
        expect(beforeClear).toContain('INFO  Listening on :8080');

        const clearButton = getClearButton();
        expect(clearButton).not.toHaveClass('disabled');

        act(() => {
            clearButton.click();
        });

        const afterClear = container.textContent;
        expect(afterClear).not.toContain('INFO  Starting application');
        expect(afterClear).not.toContain('INFO  Listening on :8080');

        const disabledClearButton = getClearButton();
        expect(disabledClearButton).toHaveClass('disabled');
        expect(mockCopyLogsButton).toHaveBeenLastCalledWith(logsFixture);
    });
});

describe('PodsLogsViewer reconnect handling', () => {
    let container: HTMLDivElement;
    let root: Root;
    let stream: Subject<LogEntry>;

    const entry = (content: string, first?: boolean): LogEntry => ({
        content,
        podName: 'demo-app-68b8fcf645-pkc5s',
        timeStamp: null,
        timeStampStr: '',
        last: false,
        ...(first ? {first: true} : {})
    });

    const occurrences = (text: string) => container.textContent.split(text).length - 1;

    beforeEach(() => {
        jest.clearAllMocks();
        jest.useFakeTimers();
        stream = new Subject<LogEntry>();
        mockGetContainerLogs.mockReturnValue(stream);
        container = document.createElement('div');
        document.body.appendChild(container);
        root = createRoot(container);
        act(() => {
            root.render(makeComponent());
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        jest.useRealTimers();
    });

    // bufferTime(100) batches entries, so emissions must be flushed before asserting.
    const emit = (entries: LogEntry[]) => {
        act(() => {
            entries.forEach(e => stream.next(e));
            jest.advanceTimersByTime(150);
        });
    };

    it('appends entries while the connection stays up', () => {
        emit([entry('line 1', true), entry('line 2'), entry('line 3')]);

        expect(occurrences('line 1')).toBe(1);
        expect(occurrences('line 3')).toBe(1);
    });

    it('drops the buffer when a reconnect replays the log', () => {
        emit([entry('line 1', true), entry('line 2'), entry('line 3')]);

        // reconnect: the backend replays from the start, first entry marked `first`
        emit([entry('line 1', true), entry('line 2'), entry('line 3'), entry('line 4')]);

        // each line must appear exactly once, not twice
        expect(occurrences('line 1')).toBe(1);
        expect(occurrences('line 2')).toBe(1);
        expect(occurrences('line 4')).toBe(1);
    });

    it('truncates correctly when the restart marker lands mid-batch', () => {
        emit([entry('line 1', true), entry('line 2')]);

        // bufferTime batched a trailing entry of the old connection with the reconnect
        emit([entry('trailing'), entry('line 1', true), entry('line 2')]);

        expect(occurrences('trailing')).toBe(0);
        expect(occurrences('line 1')).toBe(1);
        expect(occurrences('line 2')).toBe(1);
    });

    it('resets the logs available for copying on a reconnect', () => {
        emit([entry('line 1', true), entry('line 2')]);
        emit([entry('line 1', true), entry('line 2'), entry('line 3')]);

        expect(mockCopyLogsButton).toHaveBeenLastCalledWith([entry('line 1', true), entry('line 2'), entry('line 3')]);
    });

    it('keeps streaming after a non-JSON stream error', () => {
        // loadEventSource reports a dropped connection as a plain string, so the error
        // handler must not assume a JSON body nor surface the max-pods message.
        emit([entry('line 1', true)]);

        act(() => {
            stream.error('connection got closed unexpectedly');
            jest.advanceTimersByTime(600);
        });

        expect(container.textContent).not.toContain('Max pods to view logs are reached');
        expect(occurrences('line 1')).toBe(1);
    });

    it('surfaces the max pods message instead of a blank view', () => {
        act(() => {
            stream.error({body: JSON.stringify({error: {message: 'max pods to view logs are reached. Please provide more granular query'}})});
        });

        expect(container.textContent).toContain('Max pods to view logs are reached. Please provide more granular query.');
    });
});
