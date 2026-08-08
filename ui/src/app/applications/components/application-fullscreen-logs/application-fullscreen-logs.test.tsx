import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {ApplicationFullscreenLogs} from './application-fullscreen-logs';

jest.mock('../../../shared/components', () => ({
    Spinner: () => {
        const ReactLib = require('react');
        return ReactLib.createElement('div', null, 'loading spinner');
    }
}));

jest.mock('../pod-logs-viewer/pod-logs-viewer', () => {
    const ReactLib = require('react');
    return {PodsLogsViewer: () => ReactLib.createElement('div', null, 'pod logs viewer')};
});

const renderAt = (path: string) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <Route path='/applications/:name/:namespace/:container/logs' component={ApplicationFullscreenLogs} />
        </MemoryRouter>
    );

describe('ApplicationFullscreenLogs', () => {
    it('renders the lazy pod logs viewer through the Suspense fallback', async () => {
        renderAt('/applications/my-app/my-namespace/main/logs');

        expect(screen.getByText('loading spinner')).toBeInTheDocument();
        expect(await screen.findByText('pod logs viewer')).toBeInTheDocument();
        expect(screen.queryByText('loading spinner')).not.toBeInTheDocument();
    });
});
