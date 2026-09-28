import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import * as models from '../../../shared/models';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {StatusPanelToggle} from './status-panel-toggle';

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            updatePreferences: jest.fn()
        },
        applications: {
            getApplicationSyncWindowState: jest.fn(() => Promise.resolve({})),
            revisionMetadata: jest.fn(() => Promise.resolve({author: 'Test Author', date: '2026-01-01T00:00:00Z', message: 'Test commit message'}))
        },
        extensions: {
            getStatusPanelExtensions: jest.fn(() => [])
        }
    }
}));

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

describe('StatusPanelToggle', () => {
    beforeEach(() => jest.clearAllMocks());

    it('collapses the panel preference when expanded', () => {
        const {container} = render(<StatusPanelToggle pref={basePref} />);
        expect(screen.getByTitle('Collapse status panel')).toBeInTheDocument();
        expect(container.querySelector('.fa-chevron-up')).not.toBeNull();

        fireEvent.click(screen.getByTitle('Collapse status panel'));
        expect(services.viewPreferences.updatePreferences).toHaveBeenCalledWith({appDetails: {...basePref, hideStatusPanel: true}});
    });

    it('expands the panel preference when collapsed', () => {
        const pref = {...basePref, hideStatusPanel: true};
        const {container} = render(<StatusPanelToggle pref={pref} />);
        expect(screen.getByTitle('Expand status panel')).toBeInTheDocument();
        expect(container.querySelector('.fa-chevron-down')).not.toBeNull();

        fireEvent.click(screen.getByTitle('Expand status panel'));
        expect(services.viewPreferences.updatePreferences).toHaveBeenCalledWith({appDetails: {...pref, hideStatusPanel: false}});
    });

    it('switches the status panel between full and collapsed rendering', () => {
        let setHide: (hide: boolean) => void;
        const Harness = () => {
            const [hide, setHideState] = React.useState(false);
            setHide = setHideState;
            return (
                <>
                    <StatusPanelToggle pref={{...basePref, hideStatusPanel: hide}} />
                    <ApplicationStatusPanel application={application} collapsed={hide} />
                </>
            );
        };
        (services.viewPreferences.updatePreferences as jest.Mock).mockImplementation(change => setHide(change.appDetails.hideStatusPanel));

        const {container} = render(<Harness />);
        expect(screen.getByText('APP HEALTH')).toBeInTheDocument();

        fireEvent.click(screen.getByTitle('Collapse status panel'));
        expect(container.querySelector('.application-status-panel--collapsed')).not.toBeNull();
        expect(screen.queryByText('APP HEALTH')).toBeNull();

        fireEvent.click(screen.getByTitle('Expand status panel'));
        expect(container.querySelector('.application-status-panel--collapsed')).toBeNull();
        expect(screen.getByText('APP HEALTH')).toBeInTheDocument();
    });
});
