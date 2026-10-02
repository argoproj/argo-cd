import * as React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationStatusPanel} from './application-status-panel';
import {ApplicationSetStatusPanel} from './appset-status-panel';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getApplicationSyncWindowState: jest.fn(() => Promise.resolve({})),
            revisionMetadata: jest.fn(() =>
                Promise.resolve({
                    author: 'Test Author',
                    date: '2026-01-01T00:00:00Z',
                    message: 'Test commit message'
                })
            ),
            listApplicationSets: jest.fn(() => Promise.resolve({items: []}))
        },
        extensions: {
            getStatusPanelExtensions: jest.fn(() => [])
        }
    }
}));

const application = {
    metadata: {name: 'test-app', namespace: 'argocd'},
    spec: {
        project: 'default',
        source: {repoURL: 'https://github.com/org/repo.git', path: '.', targetRevision: 'HEAD'},
        destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}
    },
    status: {
        sync: {status: 'OutOfSync', revision: 'abc123def456'},
        health: {status: 'Healthy'},
        conditions: [{type: 'SharedResourceWarning', message: 'shared resource', lastTransitionTime: '2026-01-01T00:00:00Z'}],
        history: []
    }
} as unknown as models.Application;

const appSet = {
    metadata: {name: 'test-appset', namespace: 'argocd'},
    spec: {},
    status: {
        conditions: [{type: 'ResourcesUpToDate', status: 'True', message: 'all good', lastTransitionTime: '2026-01-01T00:00:00Z'}]
    }
} as unknown as models.ApplicationSet;

describe('ApplicationStatusPanel', () => {
    it('renders the full panel by default', () => {
        const {container} = render(<ApplicationStatusPanel application={application} />);
        expect(screen.getByText('APP HEALTH')).toBeInTheDocument();
        expect(screen.getByText('SYNC STATUS')).toBeInTheDocument();
        expect(container.querySelector('.application-status-panel--collapsed')).toBeNull();
    });

    it('renders a slim summary when collapsed', () => {
        const {container} = render(<ApplicationStatusPanel application={application} collapsed={true} />);
        expect(container.querySelector('.application-status-panel--collapsed')).not.toBeNull();
        expect(screen.getByText('Healthy')).toBeInTheDocument();
        expect(screen.getByText('OutOfSync')).toBeInTheDocument();
        expect(screen.queryByText('APP HEALTH')).toBeNull();
        expect(screen.queryByText('SYNC STATUS')).toBeNull();
    });

    it('opens the diff when the collapsed sync status is clicked', () => {
        const showDiff = jest.fn();
        render(<ApplicationStatusPanel application={application} collapsed={true} showDiff={showDiff} />);
        fireEvent.click(screen.getByText('OutOfSync'));
        expect(showDiff).toHaveBeenCalled();
    });

    it('shows condition counts and opens conditions when clicked while collapsed', () => {
        const showConditions = jest.fn();
        render(<ApplicationStatusPanel application={application} collapsed={true} showConditions={showConditions} />);
        fireEvent.click(screen.getByText('1 Warning'));
        expect(showConditions).toHaveBeenCalled();
    });

    it('shows the source hydrator status and opens its details when clicked while collapsed', () => {
        const hydratorApp = {
            ...application,
            spec: {...application.spec, sourceHydrator: {drySource: {}, syncSource: {}}},
            status: {...application.status, sourceHydrator: {currentOperation: {phase: 'Hydrated', startedAt: '2026-01-01T00:00:00Z'}}}
        } as unknown as models.Application;
        const showHydrateOperation = jest.fn();
        render(<ApplicationStatusPanel application={hydratorApp} collapsed={true} showHydrateOperation={showHydrateOperation} />);
        fireEvent.click(screen.getByText('Hydrated'));
        expect(showHydrateOperation).toHaveBeenCalled();
    });

    it('does not show a source hydrator entry while collapsed when the app has none', () => {
        render(<ApplicationStatusPanel application={application} collapsed={true} />);
        expect(screen.queryByTitle('Source Hydrator')).toBeNull();
    });

    it('does not run the full panel loaders while it has never been expanded', () => {
        (services.applications.revisionMetadata as jest.Mock).mockClear();
        render(<ApplicationStatusPanel application={application} collapsed={true} />);
        expect(services.applications.revisionMetadata).not.toHaveBeenCalled();
    });

    it('does not refresh hidden loaders on application updates while collapsed', async () => {
        (services.applications.listApplicationSets as jest.Mock).mockClear();
        const withOwner = (app: models.Application) =>
            ({...app, metadata: {...app.metadata, ownerReferences: [{kind: 'ApplicationSet', name: 'demo-appset'}]}} as unknown as models.Application);
        const appV1 = withOwner(application);
        const appV2 = withOwner({...application, status: {...application.status, health: {status: 'Degraded'}}} as unknown as models.Application);

        const {rerender} = render(<ApplicationStatusPanel application={appV1} collapsed={false} />);
        await waitFor(() => expect(services.applications.listApplicationSets).toHaveBeenCalledTimes(1));

        rerender(<ApplicationStatusPanel application={appV1} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={appV2} collapsed={true} />);
        await waitFor(() => expect(screen.getAllByText('Degraded').length).toBeGreaterThan(0));
        expect(services.applications.listApplicationSets).toHaveBeenCalledTimes(1);

        rerender(<ApplicationStatusPanel application={appV2} collapsed={false} />);
        await waitFor(() => expect(services.applications.listApplicationSets).toHaveBeenCalledTimes(2));
    });

    it('refreshes the sync window state on re-expand even when the application is unchanged', async () => {
        (services.applications.getApplicationSyncWindowState as jest.Mock).mockClear();
        (services.applications.revisionMetadata as jest.Mock).mockClear();

        const {rerender} = render(<ApplicationStatusPanel application={application} collapsed={false} />);
        await waitFor(() => expect(services.applications.getApplicationSyncWindowState).toHaveBeenCalledTimes(1));

        rerender(<ApplicationStatusPanel application={application} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={application} collapsed={false} />);
        await waitFor(() => expect(services.applications.getApplicationSyncWindowState).toHaveBeenCalledTimes(2));

        // the revision metadata loaders stay mounted and do not reload
        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(1);
    });

    it('does not reload revision metadata while collapsed when the revision changes', async () => {
        (services.applications.revisionMetadata as jest.Mock).mockClear();
        const appV2 = {...application, status: {...application.status, sync: {status: 'Synced', revision: 'def456abc789'}}} as unknown as models.Application;

        const {rerender} = render(<ApplicationStatusPanel application={application} collapsed={false} />);
        await waitFor(() => expect(services.applications.revisionMetadata).toHaveBeenCalledWith('test-app', 'argocd', 'abc123def456', 0, null));

        rerender(<ApplicationStatusPanel application={application} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={appV2} collapsed={true} />);
        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(1);

        rerender(<ApplicationStatusPanel application={appV2} collapsed={false} />);
        await waitFor(() => expect(services.applications.revisionMetadata).toHaveBeenCalledWith('test-app', 'argocd', 'def456abc789', 0, null));
        expect(services.applications.revisionMetadata).toHaveBeenCalledTimes(2);
    });

    it('does not remount the progressive sync loader on live ownership transitions while collapsed', async () => {
        (services.applications.listApplicationSets as jest.Mock).mockClear();
        const withOwner = {
            ...application,
            metadata: {...application.metadata, ownerReferences: [{kind: 'ApplicationSet', name: 'demo-appset'}]}
        } as unknown as models.Application;

        const {rerender} = render(<ApplicationStatusPanel application={withOwner} collapsed={false} />);
        await waitFor(() => expect(services.applications.listApplicationSets).toHaveBeenCalledTimes(1));

        rerender(<ApplicationStatusPanel application={withOwner} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={application} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={withOwner} collapsed={true} />);
        expect(services.applications.listApplicationSets).toHaveBeenCalledTimes(1);
    });

    it('does not remount the hydrator metadata loader on live hydrator transitions while collapsed', async () => {
        (services.applications.revisionMetadata as jest.Mock).mockClear();
        const hydratorApp = {
            ...application,
            spec: {...application.spec, sourceHydrator: {drySource: {}, syncSource: {}}},
            status: {
                ...application.status,
                sourceHydrator: {
                    currentOperation: {
                        phase: 'Hydrated',
                        startedAt: '2026-01-01T00:00:00Z',
                        drySHA: 'dry123',
                        sourceHydrator: {
                            drySource: {repoURL: 'https://github.com/org/repo.git', targetRevision: 'main', path: '.'},
                            syncSource: {targetBranch: 'env/test', path: '.'}
                        }
                    }
                }
            }
        } as unknown as models.Application;
        const withoutOperation = {...hydratorApp, status: {...hydratorApp.status, sourceHydrator: {}}} as unknown as models.Application;
        const dryCalls = () => (services.applications.revisionMetadata as jest.Mock).mock.calls.filter(call => call[2] === 'dry123').length;

        const {rerender} = render(<ApplicationStatusPanel application={hydratorApp} collapsed={false} />);
        await waitFor(() => expect(dryCalls()).toBe(1));

        rerender(<ApplicationStatusPanel application={hydratorApp} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={withoutOperation} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={hydratorApp} collapsed={true} />);
        expect(dryCalls()).toBe(1);
    });

    it('does not remount the last-sync metadata loader on live operation transitions while collapsed', async () => {
        (services.applications.revisionMetadata as jest.Mock).mockClear();
        const opApp = {
            ...application,
            status: {
                ...application.status,
                operationState: {phase: 'Succeeded', startedAt: '2026-01-01T00:00:00Z', finishedAt: '2026-01-01T00:01:00Z', syncResult: {revision: 'op12345'}}
            }
        } as unknown as models.Application;
        const opCalls = () => (services.applications.revisionMetadata as jest.Mock).mock.calls.filter(call => call[2] === 'op12345').length;

        const {rerender} = render(<ApplicationStatusPanel application={opApp} collapsed={false} />);
        await waitFor(() => expect(opCalls()).toBe(1));

        rerender(<ApplicationStatusPanel application={opApp} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={application} collapsed={true} />);
        rerender(<ApplicationStatusPanel application={opApp} collapsed={true} />);
        expect(opCalls()).toBe(1);
    });
});

describe('ApplicationSetStatusPanel', () => {
    it('renders the full panel by default', () => {
        const {container} = render(<ApplicationSetStatusPanel appSet={appSet} />);
        expect(screen.getByText('APPSET HEALTH')).toBeInTheDocument();
        expect(container.querySelector('.application-status-panel--collapsed')).toBeNull();
    });

    it('renders a slim summary when collapsed', () => {
        const {container} = render(<ApplicationSetStatusPanel appSet={appSet} collapsed={true} />);
        expect(container.querySelector('.application-status-panel--collapsed')).not.toBeNull();
        expect(screen.getByText('Healthy')).toBeInTheDocument();
        expect(screen.queryByText('APPSET HEALTH')).toBeNull();
    });

    it('opens conditions when the collapsed condition count is clicked', () => {
        const showConditions = jest.fn();
        render(<ApplicationSetStatusPanel appSet={appSet} collapsed={true} showConditions={showConditions} />);
        fireEvent.click(screen.getByText('1 Info'));
        expect(showConditions).toHaveBeenCalled();
    });
});
