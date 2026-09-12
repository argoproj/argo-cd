import * as React from 'react';
import {render} from '@testing-library/react';
import * as models from '../../../shared/models';
import {ApplicationStatusPanel} from './application-status-panel';

jest.mock('./revision-metadata-panel', () => ({
    RevisionMetadataPanel: (props: any) => (
        <div data-testid='revision-metadata-panel' data-version-id={props.versionId === null ? 'null' : props.versionId}>
            {props.revision}
        </div>
    )
}));

jest.mock('../../../shared/services', () => ({
    services: {
        extensions: {
            getStatusPanelExtensions: jest.fn(() => [])
        },
        applications: {
            getApplicationSyncWindowState: jest.fn(() => Promise.resolve({}))
        }
    }
}));

describe('ApplicationStatusPanel', () => {
    it('uses current source for sync status revision metadata and historical version for last sync', () => {
        const application = {
            metadata: {
                name: 'test-app',
                namespace: 'default'
            },
            spec: {
                source: {
                    repoURL: 'oci://new-repo',
                    targetRevision: 'latest'
                },
                syncPolicy: {
                    automated: {
                        enabled: false
                    }
                }
            },
            status: {
                health: {
                    status: models.HealthStatuses.Healthy
                },
                sync: {
                    status: models.SyncStatuses.Synced,
                    revision: 'BBBB'
                },
                history: [
                    {
                        id: 1,
                        revision: 'AAAA',
                        source: {
                            repoURL: 'oci://old-repo',
                            targetRevision: 'AAAA'
                        },
                        deployedAt: '2026-01-01T00:00:00Z'
                    }
                ],
                conditions: [],
                operationState: {
                    phase: models.OperationPhases.Succeeded,
                    syncResult: {
                        revision: 'AAAA'
                    },
                    finishedAt: '2026-01-01T00:00:00Z'
                }
            }
        } as models.Application;

        render(<ApplicationStatusPanel application={application} />);

        const panels = document.querySelectorAll('[data-testid="revision-metadata-panel"]');

        expect(panels).toHaveLength(2);

        // SYNC STATUS: current revision must use the current source.
        expect(panels[0]).toHaveAttribute('data-version-id', 'null');
        expect(panels[0]).toHaveTextContent('BBBB');

        // LAST SYNC: historical revision must continue using its historical version.
        expect(panels[1]).toHaveAttribute('data-version-id', '1');
        expect(panels[1]).toHaveTextContent('AAAA');
    });
});
