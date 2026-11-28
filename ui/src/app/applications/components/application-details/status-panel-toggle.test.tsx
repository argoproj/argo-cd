import {AppContext, AppContextReact} from 'argo-ui';
import * as React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as models from '../../../shared/models';
import {SHORT_WINDOW_MEDIA_QUERY} from '../../../shared/hooks/use-short-window';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {StatusPanelToggle, StatusPanelWithToggle} from './status-panel-toggle';

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            updatePreferences: jest.fn()
        },
        applications: {
            getApplicationSyncWindowState: jest.fn(),
            revisionMetadata: jest.fn()
        },
        extensions: {
            getStatusPanelExtensions: jest.fn()
        }
    }
}));

const mockMatchMedia = (matches: boolean) => {
    const listeners = new Set<(event: MediaQueryListEvent) => void>();
    const mediaQuery = {
        matches,
        addEventListener: jest.fn((_event: string, listener: (event: MediaQueryListEvent) => void) => listeners.add(listener)),
        removeEventListener: jest.fn((_event: string, listener: (event: MediaQueryListEvent) => void) => listeners.delete(listener))
    };
    window.matchMedia = jest.fn().mockImplementation((query: string) => {
        expect(query).toBe(SHORT_WINDOW_MEDIA_QUERY);
        return mediaQuery;
    });
    return mediaQuery;
};

const basePref = {hideStatusPanel: false} as AppDetailsPreferences;

const application = {
    metadata: {name: 'test-app', namespace: 'argocd'},
    spec: {
        project: 'default',
        source: {repoURL: 'https://github.com/org/repo.git', path: '.', targetRevision: 'HEAD'},
        destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}
    },
    status: {
        sync: {status: 'Synced', revision: 'abc123def456'},
        health: {status: 'Healthy'},
        history: []
    }
} as unknown as models.Application;

const show = jest.fn();
const appContext = {apis: {notifications: {show}}} as unknown as AppContext;

const renderWithContext = (ui: React.ReactElement) => render(<AppContextReact.Provider value={appContext}>{ui}</AppContextReact.Provider>);

describe('StatusPanelToggle', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockMatchMedia(false);
        (services.applications.getApplicationSyncWindowState as jest.Mock).mockResolvedValue({});
        (services.applications.revisionMetadata as jest.Mock).mockResolvedValue({author: 'Test Author', date: '2026-01-01T00:00:00Z', message: 'Test commit message'});
        (services.extensions.getStatusPanelExtensions as jest.Mock).mockReturnValue([]);
    });

    it('collapses the panel preference when expanded', () => {
        const onToggle = jest.fn();
        const {container} = render(<StatusPanelToggle collapsed={false} onToggle={onToggle} />);
        expect(screen.getByTitle('Collapse status panel')).toBeInTheDocument();
        expect(container.querySelector('.fa-chevron-up')).not.toBeNull();

        fireEvent.click(screen.getByTitle('Collapse status panel'));
        expect(onToggle).toHaveBeenCalledTimes(1);
    });

    it('expands the panel preference when collapsed', () => {
        const onToggle = jest.fn();
        const {container} = render(<StatusPanelToggle collapsed={true} onToggle={onToggle} />);
        expect(screen.getByTitle('Expand status panel')).toBeInTheDocument();
        expect(container.querySelector('.fa-chevron-down')).not.toBeNull();

        fireEvent.click(screen.getByTitle('Expand status panel'));
        expect(onToggle).toHaveBeenCalledTimes(1);
    });

    it('switches the status panel between full and collapsed rendering', () => {
        let setHide: (hide: boolean) => void;
        const Harness = () => {
            const [hide, setHideState] = React.useState(false);
            setHide = setHideState;
            return (
                <StatusPanelWithToggle pref={{...basePref, hideStatusPanel: hide}}>
                    {collapsed => <ApplicationStatusPanel application={application} collapsed={collapsed} />}
                </StatusPanelWithToggle>
            );
        };
        (services.viewPreferences.updatePreferences as jest.Mock).mockImplementation(change => setHide(change.appDetails.hideStatusPanel));

        const {container} = renderWithContext(<Harness />);
        expect(screen.getByText('APP HEALTH')).toBeVisible();
        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(1);

        fireEvent.click(screen.getByTitle('Collapse status panel'));
        expect(container.querySelector('.application-status-panel--collapsed')).not.toBeNull();
        expect(screen.getByText('APP HEALTH')).not.toBeVisible();

        fireEvent.click(screen.getByTitle('Expand status panel'));
        expect(container.querySelector('.application-status-panel--collapsed')).toBeNull();
        expect(screen.getByText('APP HEALTH')).toBeVisible();

        // the full panel stays mounted while collapsed, so its loaders do not re-run
        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(1);
    });

    it('does not re-show the loader error toast after collapsing and re-expanding', async () => {
        (services.applications.revisionMetadata as jest.Mock).mockRejectedValue(new Error('metadata unavailable'));
        let setHide: (hide: boolean) => void;
        const Harness = () => {
            const [hide, setHideState] = React.useState(false);
            setHide = setHideState;
            return (
                <StatusPanelWithToggle pref={{...basePref, hideStatusPanel: hide}}>
                    {collapsed => <ApplicationStatusPanel application={application} collapsed={collapsed} />}
                </StatusPanelWithToggle>
            );
        };
        (services.viewPreferences.updatePreferences as jest.Mock).mockImplementation(change => setHide(change.appDetails.hideStatusPanel));

        renderWithContext(<Harness />);
        await waitFor(() => expect(show).toHaveBeenCalledTimes(1));

        fireEvent.click(screen.getByTitle('Collapse status panel'));
        fireEvent.click(screen.getByTitle('Expand status panel'));
        await waitFor(() => expect(screen.getByText('APP HEALTH')).toBeVisible());

        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(1);
        expect(show).toHaveBeenCalledTimes(1);
    });

    it('auto-collapses the status panel on a short window without writing the preference', () => {
        mockMatchMedia(true);
        const {container} = renderWithContext(
            <StatusPanelWithToggle pref={basePref}>{collapsed => <ApplicationStatusPanel application={application} collapsed={collapsed} />}</StatusPanelWithToggle>
        );

        expect(container.querySelector('.application-status-panel--collapsed')).not.toBeNull();
        expect(screen.getByTitle('Expand status panel')).toBeInTheDocument();
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();

        fireEvent.click(screen.getByTitle('Expand status panel'));
        expect(container.querySelector('.application-status-panel--collapsed')).toBeNull();
        expect(screen.getByText('APP HEALTH')).toBeVisible();
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });
});
