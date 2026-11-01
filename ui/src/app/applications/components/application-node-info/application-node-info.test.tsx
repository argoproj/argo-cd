import * as React from 'react';
import {act, render, screen} from '@testing-library/react';
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

jest.mock('../../../shared/components/yaml-editor/yaml-editor', () => ({
    YamlEditor: () => null
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

    describe('CronJob', () => {
        const cronJobNode = {kind: 'CronJob', name: 'pod-reaper', namespace: 'default', version: 'v1', group: 'batch'} as models.ResourceNode;

        const cronJobLive = (status: {lastScheduleTime?: string; lastSuccessfulTime?: string}) =>
            ({apiVersion: 'batch/v1', kind: 'CronJob', metadata: {name: 'pod-reaper', namespace: 'default'}, status}) as unknown as models.State;

        // Managed by the Application: health comes from the Application status
        const renderCronJob = async (health: models.HealthStatusCode, status: {lastScheduleTime?: string; lastSuccessfulTime?: string}) => {
            const summary = {
                kind: 'CronJob',
                name: 'pod-reaper',
                namespace: 'default',
                version: 'v1',
                group: 'batch',
                status: 'Synced',
                health: {status: health}
            } as models.ResourceStatus;
            const state = {
                targetState: {apiVersion: 'batch/v1', kind: 'CronJob'},
                normalizedLiveState: {apiVersion: 'batch/v1', kind: 'CronJob'}
            } as unknown as models.ResourceDiff;
            await act(async () => {
                render(<ApplicationNodeInfo application={application} node={cronJobNode} live={cronJobLive(status)} controlled={{summary, state}} />);
            });
        };

        // Created by another resource: health comes from the resource tree
        const renderChildCronJob = async (health: models.HealthStatusCode, status: {lastScheduleTime?: string; lastSuccessfulTime?: string}) => {
            const node = {...cronJobNode, health: {status: health}} as models.ResourceNode;
            await act(async () => {
                render(<ApplicationNodeInfo application={application} node={node} live={cronJobLive(status)} controlled={null} />);
            });
        };

        it('shows both times when the CronJob is degraded', async () => {
            await renderCronJob(models.HealthStatuses.Degraded, {lastScheduleTime: '2026-10-26T16:15:00Z', lastSuccessfulTime: '2026-10-25T18:25:41Z'});

            expect(screen.getByText('LAST SCHEDULED')).toBeInTheDocument();
            expect(screen.getByText('LAST SUCCESSFUL')).toBeInTheDocument();
        });

        it('shows both times when a CronJob created by another resource is degraded', async () => {
            await renderChildCronJob(models.HealthStatuses.Degraded, {lastScheduleTime: '2026-10-26T16:15:00Z', lastSuccessfulTime: '2026-10-25T18:25:41Z'});

            expect(screen.getByText('LAST SCHEDULED')).toBeInTheDocument();
            expect(screen.getByText('LAST SUCCESSFUL')).toBeInTheDocument();
        });

        it('shows only lastSuccessfulTime when the CronJob completed its last run successfully', async () => {
            await renderCronJob(models.HealthStatuses.Healthy, {lastScheduleTime: '2026-10-27T14:20:00Z', lastSuccessfulTime: '2026-10-27T14:20:07Z'});

            expect(screen.queryByText('LAST SCHEDULED')).not.toBeInTheDocument();
            expect(screen.getByText('LAST SUCCESSFUL')).toBeInTheDocument();
        });

        it('shows only lastSuccessfulTime when a CronJob created by another resource completed its last run successfully', async () => {
            await renderChildCronJob(models.HealthStatuses.Healthy, {lastScheduleTime: '2026-10-27T14:20:00Z', lastSuccessfulTime: '2026-10-27T14:20:07Z'});

            expect(screen.queryByText('LAST SCHEDULED')).not.toBeInTheDocument();
            expect(screen.getByText('LAST SUCCESSFUL')).toBeInTheDocument();
        });

        it('shows only lastSuccessfulTime when the CronJob is running', async () => {
            await renderCronJob(models.HealthStatuses.Healthy, {lastScheduleTime: '2026-10-27T14:30:00Z', lastSuccessfulTime: '2026-10-27T14:20:07Z'});

            expect(screen.queryByText('LAST SCHEDULED')).not.toBeInTheDocument();
            expect(screen.getByText('LAST SUCCESSFUL')).toBeInTheDocument();
        });

        it('shows only lastScheduleTime when the CronJob never completed successfully', async () => {
            await renderCronJob(models.HealthStatuses.Healthy, {lastScheduleTime: '2026-10-26T16:15:00Z'});

            expect(screen.getByText('LAST SCHEDULED')).toBeInTheDocument();
            expect(screen.queryByText('LAST SUCCESSFUL')).not.toBeInTheDocument();
        });

        it('shows no times when the CronJob has not been scheduled yet', async () => {
            await renderCronJob(models.HealthStatuses.Healthy, {});

            expect(screen.queryByText('LAST SCHEDULED')).not.toBeInTheDocument();
            expect(screen.queryByText('LAST SUCCESSFUL')).not.toBeInTheDocument();
        });
    });
});
