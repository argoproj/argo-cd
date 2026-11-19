import {act, cleanup, render, screen} from '@testing-library/react';
import * as React from 'react';

import {services} from './shared/services';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: React.PropsWithChildren) => children}));
jest.mock('./applications', () => ({__esModule: true, default: {component: () => <div>applications page</div>}}));
jest.mock('./settings', () => ({__esModule: true, default: {component: () => <div>settings page</div>}}));

let bannerMounts = 0;
jest.mock('./ui-banner/ui-banner', () => ({
    Banner: ({children}: React.PropsWithChildren) => {
        React.useEffect(() => {
            bannerMounts++;
        }, []);
        return children;
    }
}));

describe('App layout', () => {
    let app: typeof import('./app');

    beforeAll(() => {
        // jsdom lacks matchMedia; the theme hooks read it for the 'auto' theme.
        window.matchMedia = jest.fn().mockReturnValue({matches: false, addEventListener: jest.fn(), removeEventListener: jest.fn()});
        jest.spyOn(services.version, 'version').mockResolvedValue({Version: 'v9.9.9'} as any);
        jest.spyOn(services.authService, 'settings').mockResolvedValue({resourceViewEnabled: true, help: {}} as any);
        jest.spyOn(services.users, 'get').mockResolvedValue({loggedIn: true, username: 'admin', iss: 'argocd', groups: []});
        // Imported after the spies: app.tsx requests the version at module load.
        app = require('./app');
    });

    afterEach(cleanup);

    it('keeps the sidebar and banner mounted across sections and updates the active item', async () => {
        app.history.replace('/applications');
        render(<app.App />);
        expect(await screen.findByText('applications page')).toBeInTheDocument();
        expect(await screen.findByText('v9.9.9', {selector: '.sidebar__version'})).toBeInTheDocument();
        expect(screen.getByRole('link', {name: 'Applications'})).toHaveAttribute('aria-current', 'page');
        const versionCalls = (services.version.version as jest.Mock).mock.calls.length;
        const mounts = bannerMounts;

        act(() => app.history.push('/settings'));
        expect(await screen.findByText('settings page')).toBeInTheDocument();
        act(() => app.history.push('/applications'));
        expect(await screen.findByText('applications page')).toBeInTheDocument();
        act(() => app.history.push('/settings'));
        expect(await screen.findByText('settings page')).toBeInTheDocument();

        expect(screen.getByRole('link', {name: 'Settings'})).toHaveAttribute('aria-current', 'page');
        expect(screen.getByRole('link', {name: 'Applications'})).not.toHaveAttribute('aria-current');
        expect(bannerMounts).toBe(mounts);
        // A remounted sidebar would refetch the version (default staleTime 0).
        expect(services.version.version).toHaveBeenCalledTimes(versionCalls);
    });

    it('renders a system-level extension registered after startup inside the layout', async () => {
        app.history.replace('/applications');
        render(<app.App />);
        expect(await screen.findByText('applications page')).toBeInTheDocument();

        act(() => (window as any).extensionsAPI.registerSystemLevelExtension(() => <div>extension page</div>, 'My Extension', '/my-extension', 'fa-star'));
        act(() => app.history.push('/my-extension'));

        expect(await screen.findByText('extension page')).toBeInTheDocument();
        expect(screen.getByRole('link', {name: 'My Extension'})).toHaveAttribute('aria-current', 'page');
    });
});
