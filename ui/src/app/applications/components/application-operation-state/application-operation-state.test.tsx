import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {ApplicationOperationState} from './application-operation-state';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            terminateOperation: jest.fn()
        }
    }
}));

const baseApp: any = {
    metadata: {
        name: 'test-app',
        namespace: 'argocd'
    },
    spec: {
        source: {
            repoURL: 'https://github.com/example/repo.git',
            path: 'guestbook',
            targetRevision: 'HEAD'
        },
        destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
        project: 'default'
    },
    status: {
        resources: [],
        sync: {status: 'Synced'},
        health: {status: 'Healthy'},
        history: []
    }
};

const baseOperationState: any = {
    phase: 'Succeeded',
    message: 'successfully synced (all tasks run)',
    startedAt: '2026-05-01T05:00:00Z',
    finishedAt: '2026-05-01T05:00:05Z',
    operation: {
        sync: {revision: 'abc1234567890'},
        initiatedBy: {automated: true}
    },
    syncResult: {
        revision: 'abc1234567890',
        resources: []
    }
};

const baseOperationStateWithResources: any = {
    ...baseOperationState,
    syncResult: {
        revision: 'abc1234567890',
        resources: [
            {
                namespace: 'default',
                kind: 'Service',
                name: 'guestbook-ui',
                message: 'service/guestbook-ui created',
                status: 'Synced'
            }
        ]
    }
};

const treeText = (json: renderer.ReactTestRendererJSON | renderer.ReactTestRendererJSON[]): string => JSON.stringify(json);

describe('ApplicationOperationState', () => {
    it('renders sync details (MESSAGE, REVISION, INITIATED BY) when not deleting', () => {
        const component = renderer.create(<ApplicationOperationState application={baseApp} operationState={baseOperationState} />);
        const text = treeText(component.toJSON());
        expect(text).toContain('"MESSAGE"');
        expect(text).toContain('successfully synced (all tasks run)');
        expect(text).toContain('"FINISHED AT"');
        expect(text).toContain('"REVISION"');
        expect(text).toContain('"INITIATED BY"');
        expect(text).toContain('automated sync policy');
    });

    it('shows only delete-in-progress info during cascading deletion (issue #27597)', () => {
        const deletionTimestamp = '2026-05-01T06:00:00Z';
        const deletingApp = {
            ...baseApp,
            metadata: {
                ...baseApp.metadata,
                deletionTimestamp
            }
        };
        const component = renderer.create(<ApplicationOperationState application={deletingApp} operationState={baseOperationStateWithResources} />);
        const text = treeText(component.toJSON());

        expect(text).toContain('OPERATION');
        expect(text).toContain('Delete');
        expect(text).toContain('STARTED AT');

        expect(text).not.toContain('successfully synced (all tasks run)');
        expect(text).not.toContain('automated sync policy');
        expect(text).not.toContain('abc1234567890');
        expect(text).not.toContain('service/guestbook-ui created');
        expect(text).not.toContain('"FINISHED AT"');
        expect(text).not.toContain('"REVISION"');
        expect(text).not.toContain('"INITIATED BY"');
        expect(text).not.toContain('"RESULT"');
    });
});
