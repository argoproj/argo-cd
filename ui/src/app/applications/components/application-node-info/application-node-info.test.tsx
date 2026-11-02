import * as React from 'react';
import {render, screen} from '@testing-library/react';
import * as models from '../../../shared/models';
import {ApplicationNodeInfo} from './application-node-info';

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            getPreferences: jest.fn(() => Promise.resolve({appDetails: {resourceView: 'manifest'}})),
            updatePreferences: jest.fn()
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
    status: {sync: {status: 'OutOfSync'}, health: {status: 'Healthy'}}
} as unknown as models.Application;

const node = {kind: 'ConfigMap', name: 'extra-cm', namespace: 'default', version: 'v1', group: ''} as models.ResourceNode;

const summary = {kind: 'ConfigMap', name: 'extra-cm', namespace: 'default', version: 'v1', group: '', status: 'OutOfSync', requiresPruning: true} as models.ResourceStatus;

describe('ApplicationNodeInfo', () => {
    it('shows "Resource not found" when the resource is gone and has no desired state', async () => {
        // A resource that is not in Git (targetState is null) and has been deleted from the cluster.
        const controlled = {
            summary,
            state: {targetState: null, normalizedLiveState: {apiVersion: 'v1', kind: 'ConfigMap'}} as unknown as models.ResourceDiff
        };

        render(<ApplicationNodeInfo application={application} node={node} live={null} controlled={controlled} />);

        expect(await screen.findByText(/Resource not found in cluster/)).toBeInTheDocument();
        expect(screen.getByText(/extra-cm/, {selector: '.application-node-info__err_msg'})).toBeInTheDocument();
        expect(screen.queryByText(/Please update your resource specification/)).not.toBeInTheDocument();
    });

    it('suggests the live API version when the desired state uses a different one', async () => {
        const controlled = {
            summary,
            state: {
                targetState: {apiVersion: 'extensions/v1beta1', kind: 'ConfigMap'},
                normalizedLiveState: {apiVersion: 'v1', kind: 'ConfigMap'}
            } as unknown as models.ResourceDiff
        };

        render(<ApplicationNodeInfo application={application} node={node} live={null} controlled={controlled} />);

        expect(await screen.findByText(/extensions\/v1beta1\/ConfigMap:/)).toBeInTheDocument();
        expect(screen.getByText(/recommended syntax is v1\/ConfigMap:extra-cm/)).toBeInTheDocument();
    });
});
