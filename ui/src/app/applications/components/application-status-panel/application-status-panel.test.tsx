import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import * as models from '../../../shared/models';
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
            )
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
});

const withRolledBack = (status: Partial<models.ApplicationStatus>) =>
    ({
        ...application,
        spec: {...application.spec, syncPolicy: {automated: {prune: false, selfHeal: false, enabled: true}}},
        status: {...application.status, ...status}
    }) as unknown as models.Application;

const pauseLine = (container: HTMLElement) => {
    const icon = container.querySelector('.fa-pause-circle');
    return icon && icon.parentElement.textContent.replace(/\s+/g, ' ').trim();
};

describe('ApplicationStatusPanel rolled back revision', () => {
    it('does not mention a rolled back revision when none is recorded', () => {
        const {container} = render(<ApplicationStatusPanel application={application} />);
        expect(pauseLine(container)).toBeNull();
    });

    it('names the paused revision, abbreviated to seven characters', () => {
        const {container} = render(<ApplicationStatusPanel application={withRolledBack({rolledBackRevisions: ['9639592aa0f1e2d3c4b5a6978899aabbccddeeff']})} />);
        expect(pauseLine(container)).toBe('Auto sync skips rolled back revision 9639592 until a new revision is available.');
    });

    it('keeps the full revisions in the tooltip', () => {
        const revision = '9639592aa0f1e2d3c4b5a6978899aabbccddeeff';
        const {container} = render(<ApplicationStatusPanel application={withRolledBack({rolledBackRevisions: [revision]})} />);
        expect(container.querySelector('.fa-pause-circle').parentElement.getAttribute('title')).toBe(revision);
    });

    it('lists one revision per source for a multi-source application', () => {
        const {container} = render(
            <ApplicationStatusPanel application={withRolledBack({rolledBackRevisions: ['9639592aa0f1e2d3c4b5a6978899aabbccddeeff', '112f220bb1c2d3e4f5061728394a5b6c7d8e9f00']})} />
        );
        expect(pauseLine(container)).toBe('Auto sync skips rolled back revision 9639592, 112f220 until a new revision is available.');
    });

    it('falls back to the singular revision field', () => {
        const {container} = render(<ApplicationStatusPanel application={withRolledBack({rolledBackRevision: '112f220bb1c2d3e4f5061728394a5b6c7d8e9f00'})} />);
        expect(pauseLine(container)).toBe('Auto sync skips rolled back revision 112f220 until a new revision is available.');
    });

    it('prefers the plural field when an application carries both', () => {
        const {container} = render(
            <ApplicationStatusPanel
                application={withRolledBack({
                    rolledBackRevision: '112f220bb1c2d3e4f5061728394a5b6c7d8e9f00',
                    rolledBackRevisions: ['9639592aa0f1e2d3c4b5a6978899aabbccddeeff']
                })}
            />
        );
        expect(pauseLine(container)).toBe('Auto sync skips rolled back revision 9639592 until a new revision is available.');
    });

    it('treats an empty plural field as no record rather than falling back', () => {
        const {container} = render(<ApplicationStatusPanel application={withRolledBack({rolledBackRevisions: []})} />);
        expect(pauseLine(container)).toBeNull();
    });

    it('leaves the collapsed panel unchanged', () => {
        const {container} = render(<ApplicationStatusPanel application={withRolledBack({rolledBackRevisions: ['9639592aa0f1e2d3c4b5a6978899aabbccddeeff']})} collapsed={true} />);
        expect(pauseLine(container)).toBeNull();
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
