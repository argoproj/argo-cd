import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';
import {MemoryRouter} from 'react-router-dom';
import {of} from 'rxjs';

import {ARGO_WARNING_COLOR} from '../../../shared/components/colors';
import * as models from '../../../shared/models';
import {PodsLogsViewer} from './pod-logs-viewer';

const getContainerLogs = jest.fn();

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getContainerLogs: (...args: unknown[]) => getContainerLogs(...args)
        },
        viewPreferences: {
            getPreferences: () => require('rxjs').of({appDetails: {darkMode: false, wrapLines: false}})
        }
    }
}));

// @tippy.js/react logs a React 19 ref deprecation the moment it renders, and tooltips have nothing
// to do with what is under test here, so draw them as a plain pass-through.
jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    Tooltip: ({children}: {children: React.ReactNode}) => children
}));

const logEntry = (podName: string, lineNum: number): models.LogEntry => ({
    content: `line ${lineNum} from ${podName}`,
    timeStamp: '2026-01-01T00:00:00Z' as unknown as models.LogEntry['timeStamp'],
    timeStampStr: '2026-01-01T00:00:00Z',
    podName,
    last: false
});

const streamLogsFrom = (podNames: string[]) => getContainerLogs.mockReturnValue(of(...podNames.map(logEntry)));

const viewer = (containerName: string) => (
    <MemoryRouter>
        <PodsLogsViewer applicationName='app' applicationNamespace='argocd' namespace='argocd' containerName={containerName} kind='Pod' name='pod-a' podName='pod-a' />
    </MemoryRouter>
);

const renderViewer = (podNames: string[]) => {
    streamLogsFrom(podNames);
    return render(viewer('main'));
};

// The highlight button is the only toolbar button drawn with the highlighter icon.
const highlightButton = (container: HTMLElement) => container.querySelector('.fa-highlighter')?.closest('button');

const waitForHighlightButton = (container: HTMLElement) => waitFor(() => expect(highlightButton(container)).toBeInTheDocument());

beforeEach(() => getContainerLogs.mockReset());

test('offers the pod highlight button when the logs come from several pods', async () => {
    const {container} = renderViewer(['pod-a', 'pod-b']);

    await waitForHighlightButton(container);
});

test('hides the pod highlight button when the logs come from a single pod', async () => {
    const {container} = renderViewer(['pod-a', 'pod-a']);

    await waitFor(() => expect(screen.getByText(/line 1 from pod-a/)).toBeInTheDocument());
    expect(highlightButton(container)).toBeUndefined();
});

test('hides the pod highlight button before any logs have arrived', async () => {
    const {container} = renderViewer([]);

    // The follow button is always present, so it marks the toolbar as rendered.
    await waitFor(() => expect(container.querySelector('.fa-angles-down')).toBeInTheDocument());
    expect(highlightButton(container)).toBeUndefined();
});

test('drops the highlight when the logs it was pointing at are discarded', async () => {
    const {container, rerender} = renderViewer(['pod-a', 'pod-b']);
    await waitForHighlightButton(container);

    fireEvent.click(highlightButton(container));
    fireEvent.click(screen.getByText('pod-a'));
    expect(highlightButton(container)).toHaveStyle({backgroundColor: ARGO_WARNING_COLOR});

    // Switching container restarts the log stream, so the highlighted pod may no longer be in view.
    streamLogsFrom(['pod-c', 'pod-d']);
    rerender(viewer('sidecar'));

    await waitForHighlightButton(container);
    expect(highlightButton(container)).not.toHaveStyle({backgroundColor: ARGO_WARNING_COLOR});
});
