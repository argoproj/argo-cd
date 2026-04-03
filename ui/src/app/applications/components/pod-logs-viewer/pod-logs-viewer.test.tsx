import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {EMPTY, of} from 'rxjs';
import {act} from 'react-dom/test-utils';
import {LogEntry} from '../../../shared/models';
import {PodsLogsViewer} from './pod-logs-viewer';

const mockGetContainerLogs = jest.fn();

jest.mock('argo-ui', () => ({
    DataLoader: ({children}: {children: (data: any) => React.ReactNode}) => children({appDetails: {darkMode: false, wrapLines: false}}),
    Tooltip: ({children}: {children: React.ReactNode}) => {
        const React = require('react');
        return React.createElement(React.Fragment, null, children);
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

jest.mock('./copy-logs-button', () => ({CopyLogsButton: () => null}));
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

    beforeEach(() => {
        jest.clearAllMocks();
        container = document.createElement('div');
        document.body.appendChild(container);
    });

    afterEach(() => {
        ReactDOM.unmountComponentAtNode(container);
        container.remove();
    });

    const renderComponent = () => {
        act(() => {
            ReactDOM.render(makeComponent(), container);
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
        expect(clearButton.disabled).toBe(true);

        const beforeClick = container.textContent;
        act(() => {
            clearButton.click();
        });

        expect(container.textContent).toBe(beforeClick);
    });

    it('clears displayed logs when the clear button is clicked', () => {
        mockGetContainerLogs.mockReturnValue(of(...logsFixture));

        renderComponent();

        const beforeClear = container.textContent;
        expect(beforeClear).toContain('INFO  Starting application');
        expect(beforeClear).toContain('INFO  Listening on :8080');

        const clearButton = getClearButton();
        expect(clearButton.disabled).toBe(false);

        act(() => {
            clearButton.click();
        });

        const afterClear = container.textContent;
        expect(afterClear).not.toContain('INFO  Starting application');
        expect(afterClear).not.toContain('INFO  Listening on :8080');

        const disabledClearButton = getClearButton();
        expect(disabledClearButton.disabled).toBe(true);
    });
});
